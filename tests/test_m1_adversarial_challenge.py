"""
Milestone 1 Adversarial Challenge & Stress Harness
Empirically tests Compose syntax, environment substitution, port collision handling,
network topology, healthcheck contracts, and service naming consistency.
"""
import json
import os
import re
import socket
import subprocess
import sys
import threading
import time
from pathlib import Path
import urllib.request
import urllib.error

PROJECT_ROOT = Path(__file__).resolve().parent.parent

class ChallengeResult:
    def __init__(self, name: str):
        self.name = name
        self.passed = False
        self.severity = "INFO" # INFO, LOW, MEDIUM, HIGH, CRITICAL
        self.details = ""
        self.evidence = {}

    def fail(self, severity: str, details: str, evidence: dict = None):
        self.passed = False
        self.severity = severity
        self.details = details
        self.evidence = evidence or {}

    def ok(self, details: str = ""):
        self.passed = True
        self.severity = "PASS"
        self.details = details

def run_compose_config(env_overrides: dict) -> tuple[int, str, str]:
    """Run `docker compose config` with specific environment overrides."""
    env = os.environ.copy()
    env.update(env_overrides)
    cmd = ["docker", "compose", "config", "--format", "json"]
    proc = subprocess.run(
        cmd,
        cwd=str(PROJECT_ROOT),
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    return proc.returncode, proc.stdout, proc.stderr

# ==============================================================================
# Challenge 1: Docker Compose Syntax & Env Substitution Matrix
# ==============================================================================
def test_compose_env_substitutions() -> list[ChallengeResult]:
    results = []

    # 1.1 Default .env
    r1 = ChallengeResult("1.1 Default Environment Compose Validation")
    rc, out, err = run_compose_config({})
    if rc == 0:
        try:
            cfg = json.loads(out)
            r1.ok(f"Parsed {len(cfg.get('services', {}))} services successfully with default .env")
        except Exception as e:
            r1.fail("MEDIUM", f"Config exited 0 but failed to parse JSON output: {e}")
    else:
        r1.fail("CRITICAL", f"docker compose config failed: {err}")
    results.append(r1)

    # 1.2 Fallback ports (POSTGRES_PORT=5434, GAME_ENGINE_PORT=8082)
    r2 = ChallengeResult("1.2 Fallback Ports Compose Validation (5434, 8082)")
    rc, out, err = run_compose_config({
        "POSTGRES_PORT": "5434",
        "GAME_ENGINE_PORT": "8082",
    })
    if rc == 0:
        r2.ok("docker compose config accepted fallback ports 5434 and 8082")
    else:
        r2.fail("CRITICAL", f"docker compose config rejected fallback ports: {err}")
    results.append(r2)

    # 1.3 Full Port Remapping Matrix
    r3 = ChallengeResult("1.3 Full Custom Port Remapping Matrix")
    custom_ports = {
        "FRONTEND_PORT": "3005",
        "POSTGRES_PORT": "5438",
        "MONGO_PORT": "27019",
        "GAME_ENGINE_PORT": "8085",
        "TELEMETRY_HTTP_PORT": "8005",
        "TELEMETRY_GRPC_PORT": "50055",
    }
    rc, out, err = run_compose_config(custom_ports)
    if rc == 0:
        r3.ok("docker compose config succeeded with all custom published ports")
    else:
        r3.fail("HIGH", f"docker compose config failed under full port remapping: {err}")
    results.append(r3)

    # 1.4 Adversarial Port Substitution (Invalid non-numeric string)
    r4 = ChallengeResult("1.4 Adversarial Invalid Port Substitution")
    rc, out, err = run_compose_config({"GAME_ENGINE_PORT": "invalid-port"})
    if rc != 0:
        r4.ok(f"docker compose correctly rejected invalid port string: {err.strip()}")
    else:
        r4.fail("LOW", "docker compose accepted non-numeric port string 'invalid-port'")
    results.append(r4)

    return results

# ==============================================================================
# Challenge 2: Port Collision Decoupling & Target vs Published Port Invariant
# ==============================================================================
def test_port_collision_decoupling() -> list[ChallengeResult]:
    results = []

    # When GAME_ENGINE_PORT=8082 is set to avoid host collision:
    rc, out, err = run_compose_config({
        "GAME_ENGINE_PORT": "8082",
        "POSTGRES_PORT": "5434",
        "TELEMETRY_HTTP_PORT": "8005",
        "TELEMETRY_GRPC_PORT": "50055",
    })
    if rc != 0:
        r = ChallengeResult("2.1 Port Decoupling Analysis")
        r.fail("CRITICAL", f"Cannot inspect config: {err}")
        return [r]

    cfg = json.loads(out)
    services = cfg.get("services", {})

    # 2.1 Game Engine Port Coupling Bug
    r_ge = ChallengeResult("2.1 Game Engine Host Port vs Container Port Coupling")
    ge_svc = services.get("jokerarena-game-ms", {})
    ge_env = ge_svc.get("environment", {})
    ge_ports = ge_svc.get("ports", [])

    # Find port mapping
    published_port = None
    target_port = None
    for p in ge_ports:
        published_port = p.get("published")
        target_port = p.get("target")

    container_env_port = ge_env.get("PORT")

    if container_env_port == "8082" and target_port == 8080:
        r_ge.fail(
            "CRITICAL",
            "FATAL PORT MISMATCH: Host port fallback GAME_ENGINE_PORT=8082 was injected into "
            "container environment PORT='8082', but compose ports map published '8082' to target '8080'. "
            "The Go binary binds to :8082 inside the container, while Docker forwards host traffic to :8080. "
            "Result: Connection refused on host, broken healthcheck, and broken Nginx reverse proxy.",
            {
                "host_published_port": published_port,
                "container_target_port": target_port,
                "container_env_PORT": container_env_port,
            }
        )
    elif container_env_port == "8080" and target_port == 8080:
        r_ge.ok("Game engine container port decoupled from host port")
    else:
        r_ge.fail("HIGH", f"Unexpected port mapping: env={container_env_port}, target={target_port}")
    results.append(r_ge)

    # 2.2 Telemetry Service Port Coupling Bug
    r_te = ChallengeResult("2.2 Telemetry HTTP Port vs Container Port Coupling")
    te_svc = services.get("jokerarena-telemetry-ms", {})
    te_env = te_svc.get("environment", {})
    te_ports = te_svc.get("ports", [])

    te_published = None
    te_target = None
    for p in te_ports:
        if p.get("target") == 8000:
            te_published = p.get("published")
            te_target = p.get("target")

    te_env_port = te_env.get("HTTP_PORT")

    if te_env_port == "8005" and te_target == 8000:
        r_te.fail(
            "CRITICAL",
            "FATAL PORT MISMATCH: Host port fallback TELEMETRY_HTTP_PORT=8005 was injected into "
            "container environment HTTP_PORT='8005', but compose ports map published '8005' to target '8000'. "
            "Uvicorn binds to :8005 inside container, while Docker forwards host traffic to :8000.",
            {
                "host_published_port": te_published,
                "container_target_port": te_target,
                "container_env_HTTP_PORT": te_env_port,
            }
        )
    elif te_env_port == "8000" and te_target == 8000:
        r_te.ok("Telemetry HTTP container port decoupled from host port")
    else:
        r_te.fail("HIGH", f"Unexpected telemetry port mapping: env={te_env_port}, target={te_target}")
    results.append(r_te)

    # 2.3 Postgres Port Decoupling (Positive control)
    r_pg = ChallengeResult("2.3 PostgreSQL Port Decoupling Verification")
    pg_svc = services.get("jokerarena-game-db", {})
    pg_ports = pg_svc.get("ports", [])
    pg_pub = None
    pg_tar = None
    for p in pg_ports:
        pg_pub = p.get("published")
        pg_tar = p.get("target")

    if pg_pub == "5434" and pg_tar == 5432:
        r_pg.ok(f"Postgres port properly decoupled: host={pg_pub}, target={pg_tar}")
    else:
        r_pg.fail("MEDIUM", f"Postgres port unexpected: host={pg_pub}, target={pg_tar}")
    results.append(r_pg)

    return results

# ==============================================================================
# Challenge 3: Healthcheck Port Hardcoding & Dynamic Failure
# ==============================================================================
def test_healthcheck_and_proxy_contracts() -> list[ChallengeResult]:
    results = []

    rc, out, err = run_compose_config({"GAME_ENGINE_PORT": "8082", "TELEMETRY_HTTP_PORT": "8005"})
    cfg = json.loads(out)
    services = cfg.get("services", {})

    # 3.1 Game Engine Healthcheck Hardcoding
    r_ge_hc = ChallengeResult("3.1 Game Engine Healthcheck Command Verification")
    ge_svc = services.get("jokerarena-game-ms", {})
    hc_test = " ".join(ge_svc.get("healthcheck", {}).get("test", []))

    if "localhost:8080/health" in hc_test:
        if ge_svc.get("environment", {}).get("PORT") == "8082":
            r_ge_hc.fail(
                "CRITICAL",
                f"HEALTHCHECK FAILURE ON FALLBACK PORT: Healthcheck tests 'http://localhost:8080/health', "
                f"but PORT environment variable sets server to listen on 8082. Container will fail healthcheck!"
            )
        else:
            r_ge_hc.ok("Healthcheck tests :8080, matches default PORT")
    else:
        r_ge_hc.fail("MEDIUM", f"Unexpected healthcheck test command: {hc_test}")
    results.append(r_ge_hc)

    # 3.2 Telemetry Healthcheck Hardcoding
    r_te_hc = ChallengeResult("3.2 Telemetry Healthcheck Command Verification")
    te_svc = services.get("jokerarena-telemetry-ms", {})
    te_hc = " ".join(te_svc.get("healthcheck", {}).get("test", []))
    if "localhost:8000/health" in te_hc:
        if te_svc.get("environment", {}).get("HTTP_PORT") == "8005":
            r_te_hc.fail(
                "CRITICAL",
                f"HEALTHCHECK FAILURE ON FALLBACK PORT: Healthcheck tests 'http://localhost:8000/health', "
                f"but HTTP_PORT environment variable sets server to 8005. Container will fail healthcheck!"
            )
        else:
            r_te_hc.ok("Healthcheck tests :8000, matches default HTTP_PORT")
    results.append(r_te_hc)

    # 3.3 Postgres Healthcheck User/DB Hardcoding
    r_pg_hc = ChallengeResult("3.3 PostgreSQL Healthcheck Hardcoded Credentials")
    pg_svc = services.get("jokerarena-game-db", {})
    pg_hc = " ".join(pg_svc.get("healthcheck", {}).get("test", []))
    if "pg_isready" in pg_hc and "POSTGRES_USER" in pg_hc and "POSTGRES_DB" in pg_hc:
        r_pg_hc.ok("PostgreSQL healthcheck uses pg_isready with credentials taken from the container environment")
    else:
        r_pg_hc.fail("LOW", f"Unexpected postgres healthcheck: {pg_hc}")
    results.append(r_pg_hc)

    return results

# ==============================================================================
# Challenge 4: Nginx Proxy Upstream Consistency
# ==============================================================================
def test_nginx_upstream_consistency() -> list[ChallengeResult]:
    results = []
    nginx_conf_path = PROJECT_ROOT / "jokerarena-fe" / "nginx.conf"
    if not nginx_conf_path.exists():
        r = ChallengeResult("4.1 Nginx Config Existence")
        r.fail("CRITICAL", "nginx.conf not found")
        return [r]

    content = nginx_conf_path.read_text(encoding="utf-8")

    # Check REST proxy upstream
    r_api = ChallengeResult("4.1 Nginx /api/ Proxy Upstream")
    m_api = re.search(r"proxy_pass\s+http://([^/:]+):(\d+)(/[^;]*);", content)
    if m_api:
        host, port, path = m_api.groups()
        if host == "jokerarena-game-ms" and port == "8080" and path == "/api/":
            r_api.ok(f"Proxy pass correctly forwards to {host}:{port}{path}")
        else:
            r_api.fail("HIGH", f"Mismatched /api/ upstream: {host}:{port}{path}")
    else:
        r_api.fail("HIGH", "Could not locate proxy_pass for /api/ in nginx.conf")
    results.append(r_api)

    # Check WebSocket proxy upstream
    r_ws = ChallengeResult("4.2 Nginx /ws/ Proxy Upstream")
    m_ws = re.search(r"location /ws/\s*\{[^}]*proxy_pass\s+http://([^/:]+):(\d+)(/[^;]*);", content, re.DOTALL)
    if m_ws:
        host, port, path = m_ws.groups()
        if host == "jokerarena-game-ms" and port == "8080" and path == "/ws/":
            r_ws.ok(f"WebSocket proxy pass correctly forwards to {host}:{port}{path}")
        else:
            r_ws.fail("HIGH", f"Mismatched /ws/ upstream: {host}:{port}{path}")
    else:
        r_ws.fail("HIGH", "Could not locate proxy_pass for /ws/ in nginx.conf")
    results.append(r_ws)

    # Check Telemetry proxy upstream
    r_tel = ChallengeResult("4.3 Nginx /telemetry/ Proxy Upstream")
    m_tel = re.search(r"location /telemetry/\s*\{[^}]*proxy_pass\s+http://([^/:]+):(\d+)(/[^;]*);", content, re.DOTALL)
    if m_tel:
        host, port, path = m_tel.groups()
        if host == "jokerarena-telemetry-ms" and port == "8000" and path == "/":
            r_tel.ok(f"Telemetry proxy pass correctly forwards to {host}:{port}{path}")
        else:
            r_tel.fail("HIGH", f"Mismatched /telemetry/ upstream: {host}:{port}{path}")
    else:
        r_tel.fail("HIGH", "Could not locate proxy_pass for /telemetry/ in nginx.conf")
    results.append(r_tel)

    return results

# ==============================================================================
# Challenge 5: Empirical Runtime Simulation of Port Collision Failure
# ==============================================================================
def test_empirical_port_binding_simulation() -> list[ChallengeResult]:
    results = []

    # Test Python Uvicorn binding behavior under HTTP_PORT=8001
    r_sim = ChallengeResult("5.1 Empirical Simulation: Telemetry Service Under Custom Port")
    try:
        from fastapi import FastAPI
        from fastapi.responses import JSONResponse
        import uvicorn

        app = FastAPI()
        @app.get("/health")
        def health():
            return {"status": "ok"}

        # Resolve the actual container HTTP_PORT from docker compose config under fallback port
        rc, out, err = run_compose_config({"TELEMETRY_HTTP_PORT": "8001"})
        te_port = 8000
        if rc == 0:
            cfg = json.loads(out)
            te_env = cfg.get("services", {}).get("jokerarena-telemetry-ms", {}).get("environment", {})
            te_port = int(te_env.get("HTTP_PORT", "8000"))

        # Start uvicorn on the resolved container port
        server_config = uvicorn.Config(app, host="127.0.0.1", port=te_port, log_level="error")
        server = uvicorn.Server(server_config)
        thread = threading.Thread(target=server.run, daemon=True)
        thread.start()
        time.sleep(1)

        # Healthcheck command probes port 8000 (as hardcoded in docker-compose.yml):
        hc_failed = False
        try:
            req = urllib.request.urlopen("http://127.0.0.1:8000/health", timeout=1)
        except Exception as e:
            hc_failed = True

        server.should_exit = True

        if hc_failed:
            r_sim.fail(
                "CRITICAL",
                f"HEALTHCHECK FAILURE ON FALLBACK PORT: Container port resolved to {te_port}, "
                "but healthcheck querying http://localhost:8000/health failed!",
                {"resolved_port": te_port, "healthcheck_status": "FAILED"}
            )
        else:
            r_sim.ok(f"Decoupled port verified: Container listens on {te_port} and healthcheck passes")
    except Exception as e:
        r_sim.fail("LOW", f"Simulation error: {e}")
    results.append(r_sim)

    return results

def run_all_challenges():
    all_results = []
    all_results.extend(test_compose_env_substitutions())
    all_results.extend(test_port_collision_decoupling())
    all_results.extend(test_healthcheck_and_proxy_contracts())
    all_results.extend(test_nginx_upstream_consistency())
    all_results.extend(test_empirical_port_binding_simulation())
    return all_results

if __name__ == "__main__":
    results = run_all_challenges()
    print("=" * 80)
    print("JOKERARENA MILESTONE 1 ADVERSARIAL CHALLENGE SUITE RESULTS")
    print("=" * 80)
    failed = [r for r in results if not r.passed]
    for r in results:
        status_sym = "[FAIL]" if not r.passed else "[PASS]"
        print(f"{status_sym} [{r.severity:8s}] {r.name}")
        if r.details:
            print(f"       Details: {r.details}")
        if r.evidence:
            print(f"       Evidence: {r.evidence}")
    print("=" * 80)
    print(f"Summary: {len(results)} tests run, {len(failed)} challenges triggered / failed.")
    print("=" * 80)
    sys.exit(1 if failed else 0)
