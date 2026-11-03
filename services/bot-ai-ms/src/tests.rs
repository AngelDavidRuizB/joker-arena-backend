use super::*;
use crate::poker::{best_play, evaluate, score, HandType};
use crate::strategy::discard_indices;
use axum::body::Body;
use axum::http::Request;
use http_body_util::BodyExt;
use tower::ServiceExt;

fn cards(s: &str) -> Vec<Card> {
    s.split_whitespace()
        .map(|c| {
            let (rank, suit) = c.split_at(c.len() - 1);
            let suit = match suit { "S" => "SPADES", "H" => "HEARTS", "C" => "CLUBS", _ => "DIAMONDS" };
            Card { rank: rank.to_string(), suit: suit.to_string() }
        })
        .collect()
}

#[test]
fn evaluates_every_hand_type() {
    let cases = [
        ("KS KH KC 4D 4S", HandType::FullHouse),
        ("2H 3H 4H 5H 6H", HandType::StraightFlush),
        ("AS 2D 3C 4H 5S", HandType::Straight),
        ("2H 7H 9H JH KH", HandType::Flush),
        ("9S 9H 9C 9D 2S", HandType::FourOfAKind),
        ("9S 9H 9C 2D 5S", HandType::ThreeOfAKind),
        ("9S 9H 2C 2D 5S", HandType::TwoPair),
        ("9S 9H", HandType::Pair),
        ("AS", HandType::HighCard),
    ];
    for (hand, want) in cases {
        assert_eq!(evaluate(&cards(hand)).unwrap().hand_type, want, "{hand}");
    }
    assert!(evaluate(&[]).is_none());
}

#[test]
fn default_ruleset_scores_the_canonical_1248_like_the_engine() {
    let s = score(&cards("KS KH KC 4D 4S"), 0, &Ruleset::default()).unwrap();
    assert_eq!((s.chips, s.mult, s.score), (78, 16, 1248));
    let s = score(&cards("KS KH KC 4D 4S"), 2, &Ruleset::default()).unwrap();
    assert_eq!(s.score, 2208);
}

#[test]
fn ruleset_changes_are_applied() {
    let mut rs = Ruleset::default();
    rs.happy_hour.enabled = true;
    rs.happy_hour.global_mult = 2;
    assert_eq!(score(&cards("KS KH KC 4D 4S"), 0, &rs).unwrap().score, 2496);
    let mut rs = Ruleset::default();
    rs.jokers.duo.enabled = false;
    assert_eq!(score(&cards("KS KH KC 4D 4S"), 0, &rs).unwrap().score, 624);
}

#[test]
fn best_play_finds_the_full_house_inside_eight_cards() {
    let hand = cards("2C KS 7D KH 4D KC 4S 9H");
    let (idx, s) = best_play(&hand, 0, &Ruleset::default()).unwrap();
    assert_eq!(s.hand_type, HandType::FullHouse);
    assert_eq!(idx, vec![1, 3, 4, 5, 6]);
}

#[test]
fn strategies_by_difficulty() {
    let weak = cards("2C 9S 7D KH 4D 3C 5S JH"); // solo carta alta (no hay escalera: falta el 6)
    let rs = Ruleset::default();
    assert_eq!(decide(&weak, 5, 2, Difficulty::Normal, &rs).action, "DISCARD");
    assert_eq!(decide(&weak, 5, 2, Difficulty::Easy, &rs).action, "PLAY");
    assert_eq!(decide(&weak, 5, 0, Difficulty::Normal, &rs).action, "PLAY"); // sin descartes
    let trips = cards("9S 9H 9C 2D 5S 7C JD KH");
    assert_eq!(decide(&trips, 5, 2, Difficulty::Normal, &rs).action, "PLAY");
    assert_eq!(decide(&trips, 5, 2, Difficulty::Hard, &rs).action, "DISCARD");
    assert_eq!(decide(&trips, 1, 2, Difficulty::Hard, &rs).action, "PLAY"); // última mano
}

#[test]
fn discards_keep_the_largest_rank_group() {
    let hand = cards("9S 9H 2C 3D 4S AC KD QH");
    let d = discard_indices(&hand);
    assert_eq!(d, vec![2, 3, 4, 6, 7]); // 2,3,4,Q,K: los más bajos fuera del par de 9
    assert!(!d.contains(&0) && !d.contains(&1));
}

async fn post(body: serde_json::Value) -> (StatusCode, serde_json::Value) {
    let res = app()
        .oneshot(Request::post("/bot/decide").header("content-type", "application/json").body(Body::from(body.to_string())).unwrap())
        .await
        .unwrap();
    let status = res.status();
    let bytes = res.into_body().collect().await.unwrap().to_bytes();
    (status, serde_json::from_slice(&bytes).unwrap())
}

#[tokio::test]
async fn http_contract_matches_the_engine_client() {
    let (status, body) = post(json!({
        "hand": [{"rank":"K","suit":"SPADES"},{"rank":"K","suit":"HEARTS"},{"rank":"K","suit":"CLUBS"},{"rank":"4","suit":"DIAMONDS"},{"rank":"4","suit":"SPADES"}],
        "hands_left": 10, "discards_left": 0, "target_score": 22000, "current_score": 0, "difficulty": "normal",
        "ruleset": {"version": 3, "jokers": {"banner": {"enabled": true, "chips_per_discard": 30}, "classic": {"enabled": true, "mult_bonus": 4}, "duo": {"enabled": true, "mult_factor": 2}}, "happy_hour": {"enabled": false, "global_mult": 1}}
    })).await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(body["action"], "PLAY");
    assert_eq!(body["card_indices"], json!([0, 1, 2, 3, 4]));
    assert_eq!(body["expected_score"], 1248);
}

#[tokio::test]
async fn http_rejects_bad_input_with_the_common_error_shape() {
    for bad in [
        json!({"hand": []}),
        json!({"hand": [{"rank":"1","suit":"SPADES"}]}),
        json!({"hand": [{"rank":"A","suit":"SPADES"}], "difficulty": "imposible"}),
        json!({"mano": 1}),
    ] {
        let (status, body) = post(bad).await;
        assert_eq!(status, StatusCode::BAD_REQUEST);
        assert!(body["error"].is_string() && body["message"].is_string());
    }
}
