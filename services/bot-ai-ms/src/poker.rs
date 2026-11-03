//! Evaluación de manos y puntuación con Jokers. Es un port fiel del motor en Go
//! (game-engine-ms/internal/poker) para que el bot calcule exactamente los mismos puntajes.

use serde::{Deserialize, Serialize};

pub const MAX_PLAY_CARDS: usize = 5;

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum HandType {
    HighCard,
    Pair,
    TwoPair,
    ThreeOfAKind,
    Straight,
    Flush,
    FullHouse,
    FourOfAKind,
    StraightFlush,
}

impl HandType {
    /// (chips, mult) base de cada mano.
    pub fn base(self) -> (i64, i64) {
        match self {
            HandType::HighCard => (5, 1),
            HandType::Pair => (10, 2),
            HandType::TwoPair => (20, 2),
            HandType::ThreeOfAKind => (30, 3),
            HandType::Straight => (30, 4),
            HandType::Flush => (35, 4),
            HandType::FullHouse => (40, 4),
            HandType::FourOfAKind => (60, 7),
            HandType::StraightFlush => (100, 8),
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Deserialize, Serialize)]
pub struct Card {
    pub rank: String,
    pub suit: String,
}

pub fn rank_value(rank: &str) -> Option<i64> {
    Some(match rank {
        "2" => 2, "3" => 3, "4" => 4, "5" => 5, "6" => 6, "7" => 7, "8" => 8, "9" => 9, "10" => 10,
        "J" => 11, "Q" => 12, "K" => 13, "A" => 14,
        _ => return None,
    })
}

pub fn rank_chips(rank: &str) -> i64 {
    match rank {
        "J" | "Q" | "K" => 10,
        "A" => 11,
        r => rank_value(r).unwrap_or(0),
    }
}

pub fn valid_suit(suit: &str) -> bool {
    matches!(suit, "SPADES" | "HEARTS" | "CLUBS" | "DIAMONDS")
}

#[derive(Debug, Clone)]
pub struct Evaluation {
    pub hand_type: HandType,
    pub scoring_idx: Vec<usize>,
    pub contains_pair: bool,
}

/// Reconoce la mano (1 a 5 cartas). Devuelve None si la selección no es válida.
pub fn evaluate(cards: &[Card]) -> Option<Evaluation> {
    let n = cards.len();
    if n == 0 || n > MAX_PLAY_CARDS {
        return None;
    }
    // Grupos por rango: (valor, índices), ordenados por tamaño y luego por valor.
    let mut groups: Vec<(i64, Vec<usize>)> = Vec::new();
    for (i, c) in cards.iter().enumerate() {
        let v = rank_value(&c.rank)?;
        match groups.iter_mut().find(|g| g.0 == v) {
            Some(g) => g.1.push(i),
            None => groups.push((v, vec![i])),
        }
    }
    groups.sort_by(|a, b| b.1.len().cmp(&a.1.len()).then(b.0.cmp(&a.0)));

    let all: Vec<usize> = (0..n).collect();
    let flush = n == MAX_PLAY_CARDS && cards.iter().all(|c| c.suit == cards[0].suit);
    let straight = n == MAX_PLAY_CARDS && is_straight(&groups);
    let top = groups[0].1.len();
    let contains_pair = top >= 2;
    let second = groups.get(1).map(|g| g.1.len()).unwrap_or(0);

    let build = |t: HandType, mut idx: Vec<usize>| {
        idx.sort_unstable();
        Some(Evaluation { hand_type: t, scoring_idx: idx, contains_pair })
    };
    if straight && flush {
        build(HandType::StraightFlush, all)
    } else if top == 4 {
        build(HandType::FourOfAKind, groups[0].1.clone())
    } else if top == 3 && second == 2 {
        build(HandType::FullHouse, all)
    } else if flush {
        build(HandType::Flush, all)
    } else if straight {
        build(HandType::Straight, all)
    } else if top == 3 {
        build(HandType::ThreeOfAKind, groups[0].1.clone())
    } else if top == 2 && second == 2 {
        let mut idx = groups[0].1.clone();
        idx.extend(&groups[1].1);
        build(HandType::TwoPair, idx)
    } else if top == 2 {
        build(HandType::Pair, groups[0].1.clone())
    } else {
        build(HandType::HighCard, vec![groups[0].1[0]])
    }
}

fn is_straight(groups: &[(i64, Vec<usize>)]) -> bool {
    if groups.len() != MAX_PLAY_CARDS {
        return false;
    }
    let mut v: Vec<i64> = groups.iter().map(|g| g.0).collect();
    v.sort_unstable();
    v[4] - v[0] == 4 || v == [2, 3, 4, 5, 14]
}

/// Ruleset de LiveOps (contracts/ruleset.md). Si falta en la petición se usan los valores v1.
#[derive(Debug, Clone, Deserialize)]
#[serde(default)]
pub struct Ruleset {
    pub version: i64,
    pub jokers: Jokers,
    pub happy_hour: HappyHour,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(default)]
pub struct Jokers {
    pub banner: Banner,
    pub classic: Classic,
    pub duo: Duo,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(default)]
pub struct Banner { pub enabled: bool, pub chips_per_discard: i64 }
#[derive(Debug, Clone, Deserialize)]
#[serde(default)]
pub struct Classic { pub enabled: bool, pub mult_bonus: i64 }
#[derive(Debug, Clone, Deserialize)]
#[serde(default)]
pub struct Duo { pub enabled: bool, pub mult_factor: i64 }
#[derive(Debug, Clone, Deserialize)]
#[serde(default)]
pub struct HappyHour { pub enabled: bool, pub global_mult: i64 }

impl Default for Banner { fn default() -> Self { Self { enabled: true, chips_per_discard: 30 } } }
impl Default for Classic { fn default() -> Self { Self { enabled: true, mult_bonus: 4 } } }
impl Default for Duo { fn default() -> Self { Self { enabled: true, mult_factor: 2 } } }
impl Default for HappyHour { fn default() -> Self { Self { enabled: false, global_mult: 1 } } }
impl Default for Jokers { fn default() -> Self { Self { banner: Banner::default(), classic: Classic::default(), duo: Duo::default() } } }
impl Default for Ruleset { fn default() -> Self { Self { version: 1, jokers: Jokers::default(), happy_hour: HappyHour::default() } } }

#[derive(Debug, Clone)]
pub struct Scored {
    pub hand_type: HandType,
    pub chips: i64,
    pub mult: i64,
    pub score: i64,
}

/// Misma cascada que ScoreWithRuleset en Go: Banner, Joker Clásico, The Duo y Happy Hour.
pub fn score(cards: &[Card], discards_left: i64, rs: &Ruleset) -> Option<Scored> {
    let ev = evaluate(cards)?;
    let discards = discards_left.max(0);
    let (base_chips, base_mult) = ev.hand_type.base();
    let card_chips: i64 = ev.scoring_idx.iter().map(|&i| rank_chips(&cards[i].rank)).sum();
    let mut chips = base_chips + card_chips;
    let mut mult = base_mult;
    if rs.jokers.banner.enabled && discards > 0 {
        chips += rs.jokers.banner.chips_per_discard * discards;
    }
    if rs.jokers.classic.enabled {
        mult += rs.jokers.classic.mult_bonus;
    }
    if rs.jokers.duo.enabled && ev.contains_pair {
        mult *= rs.jokers.duo.mult_factor;
    }
    if rs.happy_hour.enabled && rs.happy_hour.global_mult > 1 {
        mult *= rs.happy_hour.global_mult;
    }
    Some(Scored { hand_type: ev.hand_type, chips, mult, score: chips * mult })
}

/// Mejor subconjunto de 1 a 5 cartas (empates: el primero en orden lexicográfico de índices,
/// igual que BestPlay en Go).
pub fn best_play(hand: &[Card], discards_left: i64, rs: &Ruleset) -> Option<(Vec<usize>, Scored)> {
    fn visit(hand: &[Card], start: usize, size: usize, cur: &mut Vec<usize>, d: i64, rs: &Ruleset, best: &mut Option<(Vec<usize>, Scored)>) {
        if cur.len() == size {
            let subset: Vec<Card> = cur.iter().map(|&i| hand[i].clone()).collect();
            if let Some(s) = score(&subset, d, rs) {
                if best.as_ref().map_or(true, |b| s.score > b.1.score) {
                    *best = Some((cur.clone(), s));
                }
            }
            return;
        }
        for i in start..hand.len() {
            cur.push(i);
            visit(hand, i + 1, size, cur, d, rs, best);
            cur.pop();
        }
    }
    let mut best = None;
    for size in 1..=hand.len().min(MAX_PLAY_CARDS) {
        visit(hand, 0, size, &mut Vec::with_capacity(size), discards_left, rs, &mut best);
    }
    best
}
