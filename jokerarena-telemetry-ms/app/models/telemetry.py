from typing import List, Optional
from pydantic import BaseModel, Field


class CardModel(BaseModel):
    rank: str = Field(..., description="Card rank: 2..10, J, Q, K, A")
    suit: str = Field(..., description="Card suit: SPADES, HEARTS, CLUBS, DIAMONDS")
    chips: int = Field(..., description="Chip value")
    scored: bool = Field(default=True, description="Whether card scored in hand")


class JokerScoredModel(BaseModel):
    id: str = Field(..., description="Joker ID, e.g. banner, joker, the_duo")
    name: str = Field(..., description="Joker display name")
    effect: str = Field(default="", description="Description of triggered effect")
    chips_added: int = Field(default=0)
    mult_added: int = Field(default=0)
    mult_factor: float = Field(default=1.0)


class ScoringBreakdownModel(BaseModel):
    base_chips: int
    base_mult: int
    card_chips: int
    jokers: List[JokerScoredModel] = Field(default_factory=list)
    final_chips: int
    final_mult: int
    score: int


class HandEventCreate(BaseModel):
    match_id: str
    player_id: str
    round_number: int = Field(default=1)
    hand_number: int = Field(default=1)
    poker_hand: str
    cards_played: List[CardModel] = Field(default_factory=list)
    scoring: ScoringBreakdownModel
    discards_left: int = Field(default=0)
    accumulated_score: int = Field(default=0)
    timestamp: Optional[str] = None


class HandEventResponse(BaseModel):
    event_id: str
    match_id: str
    player_id: str
    round_number: int
    hand_number: int
    poker_hand: str
    cards_played: List[CardModel]
    scoring: ScoringBreakdownModel
    discards_left: int
    accumulated_score: int
    timestamp: str


class HandEventListResponse(BaseModel):
    total: int
    limit: int
    skip: int
    hands: List[HandEventResponse]


class IngestionResult(BaseModel):
    success: bool
    event_id: str
    message: str


class MatchSummary(BaseModel):
    match_id: str
    hands_played: int
    latest_timestamp: Optional[str] = None
    final_score: int = 0
