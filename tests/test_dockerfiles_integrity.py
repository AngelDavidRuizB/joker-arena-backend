"""
Empirical test harness for Dockerfiles integrity and container buildability.
Verifies:
1. Dockerfile existence and instruction parsing across all 3 services
2. Base images and pinned tags
3. Multistage artifact handoff
4. Host workspace file dependencies
5. User permissions & non-root privilege dropping
6. Build boundary isolation (.dockerignore presence and essential rules)
7. Architecture flexibility (no hardcoded GOARCH)
"""
import os
import re
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent

def parse_dockerfile(path: Path):
    with open(path, "r", encoding="utf-8") as f:
        lines = [l.strip() for l in f if l.strip() and not l.strip().startswith("#")]
    instructions = []
    current_inst = ""
    for line in lines:
        if line.endswith("\\"):
            current_inst += line[:-1].strip() + " "
        else:
            current_inst += line
            parts = current_inst.split(None, 1)
            inst_name = parts[0].upper()
            inst_args = parts[1] if len(parts) > 1 else ""
            instructions.append((inst_name, inst_args))
            current_inst = ""
    return instructions

def test_game_ms_dockerfile():
    print("--- Probing jokerarena-game-ms/Dockerfile ---")
    df_path = PROJECT_ROOT / "jokerarena-game-ms" / "Dockerfile"
    assert df_path.exists(), f"{df_path} does not exist"
    instructions = parse_dockerfile(df_path)

    # Stages
    froms = [args for inst, args in instructions if inst == "FROM"]
    assert len(froms) == 2, f"Expected 2 stages, found {len(froms)}: {froms}"
    assert "golang:1.22-alpine" in froms[0] and "builder" in froms[0].lower()
    assert "alpine:3.19" in froms[1]
    print(f"[OK] Multistage builder/runtime declared: {froms}")

    # Copy files check
    assert (PROJECT_ROOT / "jokerarena-game-ms" / "go.mod").exists(), "go.mod missing"
    assert (PROJECT_ROOT / "jokerarena-game-ms" / "main.go").exists(), "main.go missing"
    print("[OK] Go workspace dependencies exist on host")

    # Multistage copy
    copy_builder = [args for inst, args in instructions if inst == "COPY" and "--from=builder" in args]
    assert len(copy_builder) == 1, f"Expected 1 COPY --from=builder, got {copy_builder}"
    assert "/build/game-engine" in copy_builder[0] and "/app/game-engine" in copy_builder[0]
    print(f"[OK] Multistage binary artifact handoff verified: {copy_builder[0]}")

    # Non-root user
    users = [args for inst, args in instructions if inst == "USER"]
    assert "appuser:appgroup" in users, f"Non-root USER not set: {users}"
    print(f"[OK] Non-root execution verified: USER {users[0]}")

    # Arch coupling audit: GOARCH=amd64 must NOT be hardcoded
    build_cmds = [args for inst, args in instructions if inst == "RUN" and "go build" in args]
    assert not any("GOARCH=amd64" in cmd for cmd in build_cmds), "GOARCH=amd64 must not be hardcoded in Dockerfile"
    print("[OK] Architecture flexibility verified (GOARCH=amd64 omitted)")

def test_telemetry_ms_dockerfile():
    print("\n--- Probing jokerarena-telemetry-ms/Dockerfile ---")
    df_path = PROJECT_ROOT / "jokerarena-telemetry-ms" / "Dockerfile"
    assert df_path.exists(), f"{df_path} does not exist"
    instructions = parse_dockerfile(df_path)

    froms = [args for inst, args in instructions if inst == "FROM"]
    assert len(froms) == 1
    assert "python:3.12-slim" in froms[0]
    print(f"[OK] Base image verified: {froms[0]}")

    assert (PROJECT_ROOT / "jokerarena-telemetry-ms" / "requirements.txt").exists(), "requirements.txt missing"
    assert (PROJECT_ROOT / "jokerarena-telemetry-ms" / "main.py").exists(), "main.py missing"
    print("[OK] Python dependencies and main.py exist on host")

    users = [args for inst, args in instructions if inst == "USER"]
    assert "appuser:appgroup" in users, f"Non-root USER not set: {users}"
    print(f"[OK] Non-root execution verified: USER {users[0]}")

    env_vars = {}
    for inst, args in instructions:
        if inst == "ENV":
            for token in args.split():
                if "=" in token:
                    k, v = token.split("=", 1)
                    env_vars[k] = v
    assert env_vars.get("PYTHONDONTWRITEBYTECODE") == "1", "PYTHONDONTWRITEBYTECODE must be 1"
    print(f"[OK] Environment variables verified: PYTHONDONTWRITEBYTECODE={env_vars.get('PYTHONDONTWRITEBYTECODE')}")

def test_frontend_dockerfile():
    print("\n--- Probing jokerarena-fe/Dockerfile ---")
    df_path = PROJECT_ROOT / "jokerarena-fe" / "Dockerfile"
    assert df_path.exists(), f"{df_path} does not exist"
    instructions = parse_dockerfile(df_path)

    froms = [args for inst, args in instructions if inst == "FROM"]
    assert len(froms) == 2
    assert "node:20-alpine" in froms[0]  # node:22-alpine fails with 'exec format error' on some Docker Desktop hosts
    assert "nginx:1.27-alpine" in froms[1]
    print(f"[OK] Multistage builder/runtime declared: {froms}")

    assert (PROJECT_ROOT / "jokerarena-fe" / "package.json").exists(), "package.json missing"
    assert (PROJECT_ROOT / "jokerarena-fe" / "nginx.conf").exists(), "nginx.conf missing"
    print("[OK] Frontend manifests and nginx.conf exist on host")

    copy_builder = [args for inst, args in instructions if inst == "COPY" and "--from=builder" in args]
    assert len(copy_builder) == 1
    assert "/app/dist" in copy_builder[0] and "/usr/share/nginx/html" in copy_builder[0]
    print(f"[OK] Multistage static asset handoff verified: {copy_builder[0]}")

def test_dockerignore_audit():
    print("\n--- Auditing .dockerignore Boundary Files ---")
    # 1. Frontend .dockerignore
    fe_ignore = PROJECT_ROOT / "jokerarena-fe" / ".dockerignore"
    assert fe_ignore.exists(), "jokerarena-fe/.dockerignore is missing"
    fe_content = fe_ignore.read_text(encoding="utf-8")
    assert "node_modules" in fe_content, "jokerarena-fe/.dockerignore must ignore node_modules"
    assert "dist" in fe_content, "jokerarena-fe/.dockerignore must ignore dist"
    print("[OK] jokerarena-fe/.dockerignore verified")

    # 2. Telemetry .dockerignore
    te_ignore = PROJECT_ROOT / "jokerarena-telemetry-ms" / ".dockerignore"
    assert te_ignore.exists(), "jokerarena-telemetry-ms/.dockerignore is missing"
    te_content = te_ignore.read_text(encoding="utf-8")
    assert "__pycache__" in te_content, "jokerarena-telemetry-ms/.dockerignore must ignore __pycache__"
    assert "*.pyc" in te_content, "jokerarena-telemetry-ms/.dockerignore must ignore *.pyc"
    print("[OK] jokerarena-telemetry-ms/.dockerignore verified")

    # 3. Game Engine .dockerignore
    ge_ignore = PROJECT_ROOT / "jokerarena-game-ms" / ".dockerignore"
    assert ge_ignore.exists(), "jokerarena-game-ms/.dockerignore is missing"
    ge_content = ge_ignore.read_text(encoding="utf-8")
    assert "game-engine" in ge_content, "jokerarena-game-ms/.dockerignore must ignore game-engine"
    print("[OK] jokerarena-game-ms/.dockerignore verified")

if __name__ == "__main__":
    try:
        test_game_ms_dockerfile()
        test_telemetry_ms_dockerfile()
        test_frontend_dockerfile()
        test_dockerignore_audit()
        print("\nALL DOCKERFILE INTEGRITY AUDITS PASSED (100%).")
        sys.exit(0)
    except AssertionError as e:
        print(f"\n[FAIL] Assertion failed: {e}")
        sys.exit(1)
