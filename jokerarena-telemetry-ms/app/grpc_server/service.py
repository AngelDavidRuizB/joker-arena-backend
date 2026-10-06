import logging
import uuid
from datetime import datetime, timezone
from typing import Optional

import grpc

try:
    from proto import telemetry_pb2, telemetry_pb2_grpc
except ImportError:
    from app.grpc_server import telemetry_pb2, telemetry_pb2_grpc

from app.db.mongo import MongoManager, mongo_manager

logger = logging.getLogger("telemetry.grpc")


class TelemetryServiceServicer(telemetry_pb2_grpc.TelemetryServiceServicer):
    """
    gRPC Servant implementing TelemetryService for Game Engine (Go) poker events.
    """

    def __init__(self, manager: Optional[MongoManager] = None):
        self.mongo = manager or mongo_manager

    async def HealthCheck(self, request, context):
        """Asynchronously checks service health and responds."""
        try:
            await self.mongo.ping()
        except Exception as exc:
            logger.warning(f"HealthCheck ping warning: {exc}")
        return telemetry_pb2.HealthResponse(status="ok")

    async def RecordHand(self, request, context):
        """Processes and stores a hand event in MongoDB or in-memory fallback."""
        try:
            event_id = str(uuid.uuid4())
            ts = request.timestamp or datetime.now(timezone.utc).isoformat()

            cards = [
                {
                    "rank": c.rank,
                    "suit": c.suit,
                    "chips": c.chips,
                    "scored": c.scored,
                }
                for c in request.cards_played
            ]

            jokers = []
            scoring_dict = {}
            if request.HasField("scoring"):
                jokers = [
                    {
                        "id": j.id,
                        "name": j.name,
                        "effect": j.effect,
                        "chips_added": j.chips_added,
                        "mult_added": j.mult_added,
                        "mult_factor": j.mult_factor,
                    }
                    for j in request.scoring.jokers
                ]
                scoring_dict = {
                    "base_chips": request.scoring.base_chips,
                    "base_mult": request.scoring.base_mult,
                    "card_chips": request.scoring.card_chips,
                    "jokers": jokers,
                    "final_chips": request.scoring.final_chips,
                    "final_mult": request.scoring.final_mult,
                    "score": request.scoring.score,
                }

            doc = {
                "event_id": event_id,
                "match_id": request.match_id,
                "player_id": request.player_id,
                "round_number": request.round_number,
                "hand_number": request.hand_number,
                "poker_hand": request.poker_hand,
                "cards_played": cards,
                "scoring": scoring_dict,
                "discards_left": request.discards_left,
                "accumulated_score": request.accumulated_score,
                "timestamp": ts,
                "created_at": datetime.now(timezone.utc).isoformat(),
            }

            recorded_id = await self.mongo.record_event(doc)
            final_event_id = recorded_id or event_id
            logger.info(
                f"Recorded hand event {final_event_id} for match {request.match_id} (poker_hand: {request.poker_hand})"
            )

            return telemetry_pb2.HandEventResponse(
                success=True,
                event_id=final_event_id,
                message="Hand event recorded successfully",
            )
        except Exception as exc:
            logger.error(f"Unexpected error in RecordHand: {exc}", exc_info=True)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(exc))
            return telemetry_pb2.HandEventResponse(
                success=False,
                event_id="",
                message=f"Internal error: {str(exc)}",
            )
