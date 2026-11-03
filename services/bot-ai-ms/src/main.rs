//! bot-ai-ms: decide la jugada del oponente bot. Servicio sin estado.
mod poker;
mod strategy;

use axum::{
    extract::rejection::JsonRejection,
    http::StatusCode,
    response::{IntoResponse, Response},
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use serde_json::json;

use poker::{valid_suit, rank_value, Card, Ruleset};
use strategy::{decide, Difficulty};

#[derive(Debug, Deserialize)]
pub struct DecideRequest {
    pub hand: Vec<Card>,
    #[serde(default = "default_hands")]
    pub hands_left: i64,
    #[serde(default)]
    pub discards_left: i64,
    #[serde(default)]
    pub target_score: i64,
    #[serde(default)]
    pub current_score: i64,
    #[serde(default)]
    pub difficulty: String,
    #[serde(default)]
    pub ruleset: Option<Ruleset>,
}

fn default_hands() -> i64 {
    1
}

#[derive(Debug, Serialize)]
pub struct DecideResponse {
    pub action: &'static str,
    pub card_indices: Vec<usize>,
    pub difficulty: &'static str,
    pub expected_hand: Option<poker::HandType>,
    pub expected_score: Option<i64>,
}

fn error(status: StatusCode, code: &str, message: &str) -> Response {
    (status, Json(json!({ "error": code, "message": message }))).into_response()
}

async fn health() -> Json<serde_json::Value> {
    Json(json!({ "status": "ok", "service": "bot-ai-ms", "deps": {} }))
}

async fn decide_handler(payload: Result<Json<DecideRequest>, JsonRejection>) -> Response {
    let Json(req) = match payload {
        Ok(p) => p,
        Err(e) => return error(StatusCode::BAD_REQUEST, "invalid_request", &e.body_text()),
    };
    if req.hand.is_empty() || req.hand.len() > 8 {
        return error(StatusCode::BAD_REQUEST, "invalid_hand", "La mano debe tener entre 1 y 8 cartas.");
    }
    if req.hand.iter().any(|c| rank_value(&c.rank).is_none() || !valid_suit(&c.suit)) {
        return error(StatusCode::BAD_REQUEST, "invalid_card", "Carta inválida: rango 2-10,J,Q,K,A y palo SPADES|HEARTS|CLUBS|DIAMONDS.");
    }
    let Some(difficulty) = Difficulty::parse(&req.difficulty) else {
        return error(StatusCode::BAD_REQUEST, "invalid_difficulty", "difficulty debe ser easy, normal o hard.");
    };
    let rs = req.ruleset.unwrap_or_default();
    let d = decide(&req.hand, req.hands_left, req.discards_left, difficulty, &rs);
    let name = match difficulty {
        Difficulty::Easy => "easy",
        Difficulty::Normal => "normal",
        Difficulty::Hard => "hard",
    };
    Json(DecideResponse { action: d.action, card_indices: d.card_indices, difficulty: name, expected_hand: d.expected_hand, expected_score: d.expected_score }).into_response()
}

pub fn app() -> Router {
    Router::new()
        .route("/health", get(health))
        .route("/bot/decide", post(decide_handler))
        .fallback(|| async { error(StatusCode::NOT_FOUND, "not_found", "Ruta no encontrada.") })
}

#[tokio::main]
async fn main() {
    if std::env::args().any(|a| a == "--healthcheck") {
        // La imagen final no trae curl/wget: el HEALTHCHECK de Docker usa este modo.
        let port = std::env::var("PORT").unwrap_or_else(|_| "8084".into());
        let ok = tokio::net::TcpStream::connect(format!("127.0.0.1:{port}")).await.is_ok();
        std::process::exit(if ok { 0 } else { 1 });
    }
    let port = std::env::var("PORT").unwrap_or_else(|_| "8084".into());
    let listener = tokio::net::TcpListener::bind(format!("0.0.0.0:{port}")).await.expect("no se pudo abrir el puerto");
    println!("bot-ai-ms escuchando en :{port}");
    axum::serve(listener, app())
        .with_graceful_shutdown(async {
            let _ = tokio::signal::ctrl_c().await;
        })
        .await
        .expect("error del servidor");
}

#[cfg(test)]
mod tests;
