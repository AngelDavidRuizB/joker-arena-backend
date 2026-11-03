# Contrato del ruleset (LiveOps)

Dueño: liveops-rules-ms. Consumidor: game-engine-ms (y bot-ai-ms lo recibe en cada petición).

```json
{
  "version": 1,
  "updated_at": "2026-11-03T00:00:00Z",
  "updated_by": "seed",
  "jokers": {
    "banner":  { "enabled": true, "chips_per_discard": 30 },
    "classic": { "enabled": true, "mult_bonus": 4 },
    "duo":     { "enabled": true, "mult_factor": 2 }
  },
  "happy_hour": { "enabled": false, "global_mult": 1 }
}
```

| Campo | Tipo | Rango válido | Efecto en la puntuación |
|---|---|---|---|
| `jokers.banner.chips_per_discard` | entero | 0 – 200 | +N chips por cada descarte restante |
| `jokers.classic.mult_bonus` | entero | 0 – 50 | +N mult en toda mano |
| `jokers.duo.mult_factor` | entero | 1 – 10 | ×N mult si la mano contiene un par |
| `jokers.*.enabled` | booleano | — | si es `false`, el Joker aparece con `triggered: false` y aporta 0 |
| `happy_hour.global_mult` | entero | 1 – 5 | ×N al mult final, solo si `happy_hour.enabled` |
| `version` | entero | ≥ 1, creciente | lo asigna liveops-rules-ms |

La **versión 1** es exactamente el comportamiento del Prototipo 1 (la mano canónica del test sigue puntuando 1248).

## API de liveops-rules-ms (a través del gateway con prefijo `/admin`)

| Método | Ruta interna | Descripción |
|---|---|---|
| GET | `/liveops/ruleset` | Ruleset vigente. |
| PUT | `/liveops/ruleset` | Body: `{ "expected_version": 1, "jokers": {...}, "happy_hour": {...} }`. 409 si `expected_version` no es la vigente; 400 si un valor está fuera de rango. |
| POST | `/liveops/jokers/{banner\|classic\|duo}/toggle` | Invierte `enabled` de ese Joker (nueva versión). |
| POST | `/liveops/happy-hour` | Body: `{ "enabled": true, "global_mult": 2 }` (nueva versión). |
| GET | `/liveops/ruleset/history` | Últimas 20 versiones. |
