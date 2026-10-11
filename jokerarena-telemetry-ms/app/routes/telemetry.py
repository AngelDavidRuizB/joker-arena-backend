import uuid
from datetime import datetime, timezone
from typing import List, Optional

from fastapi import APIRouter, HTTPException, Query, status

from app.db.mongo import mongo_manager
from app.models.telemetry import (
    HandEventCreate,
    HandEventListResponse,
    HandEventResponse,
    IngestionResult,
    MatchSummary,
)

router = APIRouter(prefix="/telemetry", tags=["Telemetry"])


def doc_to_response(doc: dict) -> HandEventResponse:
    scoring_data = doc.get("scoring")
    if not isinstance(scoring_data, dict):
        scoring_data = {}
    cards_data = doc.get("cards_played")
    if not isinstance(cards_data, list):
        cards_data = []
    return HandEventResponse(
        event_id=doc.get("event_id", str(doc.get("_id", ""))),
        match_id=doc.get("match_id", ""),
        player_id=doc.get("player_id", ""),
        round_number=doc.get("round_number", 1),
        hand_number=doc.get("hand_number", 1),
        poker_hand=doc.get("poker_hand", ""),
        cards_played=cards_data,
        scoring=scoring_data,
        discards_left=doc.get("discards_left", 0),
        accumulated_score=doc.get("accumulated_score", 0),
        timestamp=doc.get("timestamp", ""),
    )


@router.post("/hands", response_model=IngestionResult, status_code=status.HTTP_201_CREATED)
async def record_hand_rest(payload: HandEventCreate):
    """Direct REST ingestion of hand events for HTTP clients and testing."""
    event_id = str(uuid.uuid4())
    ts = payload.timestamp or datetime.now(timezone.utc).isoformat()
    doc = payload.model_dump()
    doc["event_id"] = event_id
    doc["timestamp"] = ts
    doc["created_at"] = datetime.now(timezone.utc).isoformat()

    recorded_id = await mongo_manager.record_event(doc)
    return IngestionResult(
        success=True,
        event_id=recorded_id or event_id,
        message="Hand event recorded successfully",
    )


@router.get("/hands", response_model=HandEventListResponse)
async def list_hands(
    match_id: Optional[str] = Query(None, description="Filter by match ID"),
    player_id: Optional[str] = Query(None, description="Filter by player ID"),
    poker_hand: Optional[str] = Query(None, description="Filter by poker hand"),
    limit: int = Query(50, ge=1, le=100),
    skip: int = Query(0, ge=0),
):
    """Lists recorded hand events with pagination and filters."""
    query = {}
    if match_id:
        query["match_id"] = match_id
    if player_id:
        query["player_id"] = player_id
    if poker_hand:
        query["poker_hand"] = poker_hand

    total, raw_events = await mongo_manager.get_hands(query=query, limit=limit, skip=skip)
    hands = [doc_to_response(doc) for doc in raw_events]

    return HandEventListResponse(total=total, limit=limit, skip=skip, hands=hands)


@router.get("/hands/{event_id}", response_model=HandEventResponse)
async def get_hand(event_id: str):
    """Gets a specific hand event by event_id."""
    doc = await mongo_manager.get_hand_by_id(event_id)
    if not doc:
        raise HTTPException(status_code=404, detail="Hand event not found")
    return doc_to_response(doc)


@router.get("/matches", response_model=List[MatchSummary])
async def list_matches():
    """Returns distinct matches with summary telemetry."""
    raw_summaries = await mongo_manager.get_matches_summary()
    return [MatchSummary(**item) for item in raw_summaries]


@router.get("/matches/{match_id}", response_model=List[HandEventResponse])
async def get_match_events(match_id: str):
    """Returns all hand events for a match in sequential order."""
    raw_events = await mongo_manager.get_match_events(match_id)
    return [doc_to_response(doc) for doc in raw_events]
