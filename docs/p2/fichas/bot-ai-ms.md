#### jokerarena-bot-ai-ms

- **Tipo:** Lógica de Negocio (Microservicio Autónomo de Cómputo Heurístico)
- **Lenguaje / framework:** Rust 1.83 / Axum + Tokio + Serde
- **Responsabilidad:** Evaluación heurística y determinación determinista de jugadas y descartes para el oponente simulado (Bot IA) en tiempo real según el nivel de dificultad (`easy`, `normal`, `hard`) y el ruleset activo de LiveOps.
- **Puertos / interfaces que expone:** HTTP `:8084` interno en la red `jokerarena-net` (`POST /bot/decide`, `GET /health`). No expuesto al público por el Gateway.
- **Conectores de salida:** Ninguno (servicio computacional puro, desacoplado y sin dependencias de red de salida).
- **Datos que posee:** Ninguno (servicio 100% *Stateless* / sin estado ni persistencia propia).
- **Eventos:** No publica ni consume directamente del broker; es invocado síncronamente vía HTTP por `game-engine-ms` con timeout de 500 ms y fallback automático en Go.
- **Patrones aplicados:**
  - *Stateless Service:* Permite escalado horizontal trivial e inmunidad ante caídas de sesión.
  - *Strategy Pattern:* Selección de algoritmo de decisión según dificultad (`easy` juega siempre la mejor mano; `normal` descarta manos inferiores a trío; `hard` busca color/escalera salvo en última mano).
  - *Ruleset-Aware Scoring:* Aplica los multiplicadores de *LiveOps* recibidos en el payload para estimar el valor real de la jugada.

