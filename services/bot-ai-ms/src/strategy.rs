//! Estrategia del bot por dificultad.
//!
//! - easy:   siempre juega su mejor mano (nunca descarta).
//! - normal: igual que el bot local del motor: descarta si su mejor mano es más débil que
//!           un trío y le quedan descartes.
//! - hard:   descarta si su mejor mano es más débil que un color (busca escaleras/colores),
//!           salvo en su última mano.

use crate::poker::{best_play, rank_value, Card, HandType, Ruleset, MAX_PLAY_CARDS};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Difficulty {
    Easy,
    Normal,
    Hard,
}

impl Difficulty {
    pub fn parse(s: &str) -> Option<Self> {
        match s.trim().to_ascii_lowercase().as_str() {
            "easy" => Some(Self::Easy),
            "" | "normal" => Some(Self::Normal),
            "hard" => Some(Self::Hard),
            _ => None,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Decision {
    pub action: &'static str,
    pub card_indices: Vec<usize>,
    pub expected_hand: Option<HandType>,
    pub expected_score: Option<i64>,
}

pub fn decide(hand: &[Card], hands_left: i64, discards_left: i64, difficulty: Difficulty, rs: &Ruleset) -> Decision {
    let best = best_play(hand, discards_left, rs);
    let (idx, scored) = match best {
        Some(b) => b,
        None => return Decision { action: "PLAY", card_indices: vec![0], expected_hand: None, expected_score: None },
    };
    let threshold = match difficulty {
        Difficulty::Easy => None,
        Difficulty::Normal => Some(HandType::ThreeOfAKind),
        Difficulty::Hard if hands_left > 1 => Some(HandType::Flush),
        Difficulty::Hard => None,
    };
    if let Some(t) = threshold {
        if discards_left > 0 && scored.hand_type < t {
            return Decision { action: "DISCARD", card_indices: discard_indices(hand), expected_hand: None, expected_score: None };
        }
    }
    Decision { action: "PLAY", card_indices: idx, expected_hand: Some(scored.hand_type), expected_score: Some(scored.score) }
}

/// Las 1..5 cartas de menor rango que NO forman parte del grupo de rango más grande
/// (mismo criterio que discardIndices en Go).
pub fn discard_indices(hand: &[Card]) -> Vec<usize> {
    if hand.is_empty() {
        return vec![];
    }
    let values: Vec<i64> = hand.iter().map(|c| rank_value(&c.rank).unwrap_or(0)).collect();
    let mut best_val = 0;
    let mut best_count = 0;
    for &v in &values {
        let count = values.iter().filter(|&&x| x == v).count();
        if count > best_count || (count == best_count && v > best_val) {
            best_val = v;
            best_count = count;
        }
    }
    let mut candidates: Vec<usize> = (0..hand.len()).filter(|&i| values[i] != best_val).collect();
    if candidates.is_empty() {
        candidates = (0..hand.len()).collect();
    }
    candidates.sort_by(|&a, &b| values[a].cmp(&values[b]).then(a.cmp(&b)));
    let mut out: Vec<usize> = candidates.into_iter().take(MAX_PLAY_CARDS).collect();
    out.sort_unstable();
    out
}
