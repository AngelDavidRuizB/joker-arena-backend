from typing import Any, List, Optional
from pydantic import BaseModel, Field, field_validator


class CardModel(BaseModel):
    rank: str = Field(default="", description="Card rank: 2..10, J, Q, K, A")
    suit: str = Field(default="", description="Card suit: SPADES, HEARTS, CLUBS, DIAMONDS")
    chips: int = Field(default=0, description="Chip value")
    scored: bool = Field(default=True, description="Whether card scored in hand")


class JokerScoredModel(BaseModel):
    id: str = Field(default="", description="Joker ID, e.g. banner, joker, the_duo")
    name: str = Field(default="", description="Joker display name")
    effect: str = Field(default="", description="Description of triggered effect")
    chips_added: int = Field(default=0)
    mult_added: int = Field(default=0)
    mult_factor: float = Field(default=1.0)


class ScoringBreakdownModel(BaseModel):
    base_chips: int = Field(default=0)
    base_mult: int = Field(default=0)
    card_chips: int = Field(default=0)
    jokers: List[JokerScoredModel] = Field(default_factory=list)
    final_chips: int = Field(default=0)
    final_mult: int = Field(default=0)
    score: int = Field(default=0)


class HandEventCreate(BaseModel):
    match_id: str
    player_id: str
    round_number: int = Field(default=1)
    hand_number: int = Field(default=1)
    poker_hand: str
    cards_played: List[CardModel] = Field(default_factory=list)
    scoring: ScoringBreakdownModel = Field(default_factory=ScoringBreakdownModel)
    discards_left: int = Field(default=0)
    accumulated_score: int = Field(default=0)
    timestamp: Optional[str] = None

    @field_validator("scoring", mode="before")
    @classmethod
    def validate_scoring(cls, v: Any) -> Any:
        if v is None or not isinstance(v, (dict, ScoringBreakdownModel)):
            return {}
        return v

    @field_validator("cards_played", mode="before")
    @classmethod
    def validate_cards_played(cls, v: Any) -> Any:
        if v is None or not isinstance(v, list):
            return []
        return v


class HandEventResponse(BaseModel):
    event_id: str
    match_id: str
    player_id: str
    round_number: int
    hand_number: int
    poker_hand: str
    cards_played: List[CardModel] = Field(default_factory=list)
    scoring: ScoringBreakdownModel = Field(default_factory=ScoringBreakdownModel)
    discards_left: int
    accumulated_score: int
    timestamp: str

    @field_validator("scoring", mode="before")
    @classmethod
    def validate_scoring(cls, v: Any) -> Any:
        if v is None or not isinstance(v, (dict, ScoringBreakdownModel)):
            return {}
        return v

    @field_validator("cards_played", mode="before")
    @classmethod
    def validate_cards_played(cls, v: Any) -> Any:
        if v is None or not isinstance(v, list):
            return []
        return v


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
