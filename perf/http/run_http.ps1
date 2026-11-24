# ==============================================================================
# JokerArena P3 - D1, D2 y D5 - Corre todas las pruebas de carga HTTP con k6
# Responsable: Jose Alejandro Solano Garces
#
# Requisitos: k6 instalado en Windows (winget install k6), Docker Desktop y el
# sistema levantado. Se corre desde la raiz del repositorio:
#
#   # P2 (hoy): gateway en http://localhost:8080
#   powershell -ExecutionPolicy Bypass -File perf/http/run_http.ps1
#
#   # P3 (sistema completo con reverse proxy y TLS): solo D1 sobre https
#   powershell -ExecutionPolicy Bypass -File perf/http/run_http.ps1 -BaseUrl https://localhost -Pruebas d1
#
# Parametros:
#   -BaseUrl      URL de entrada (P2 http://localhost:8080, P3 https://localhost)
#   -ComposeDir   carpeta donde esta el compose del sistema levantado y su .env
#   -ComposeFile  docker-compose.p2.yml (P2) o docker-compose.yml (P3); se detecta solo
#   -Pruebas      d1, d2, login (por defecto las tres)
#   -Vus          niveles de usuarios (por defecto 10,50,100,200)
#   -Reps         repeticiones de cada nivel en D1 (por defecto 3)
#   -RepsTls      repeticiones de cada nivel en D2 (por defecto 2)
#   -Duration     duracion de cada corrida (por defecto 30s)
#
# Salida: perf/http/resultados/<fecha>/  (un JSON de k6 por corrida, manifiesto
# con el comando exacto de cada corrida, docker stats y datos de la maquina).
# Despues: python perf/http/resumir.py perf/http/resultados/<fecha>
# ==============================================================================
param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$ComposeDir = "",
    [string]$ComposeFile = "",
    [string[]]$Pruebas = @("d1", "d2", "login"),
    [string[]]$Vus = @("10", "50", "100", "200"),
    [int]$Reps = 3,
    [int]$RepsTls = 2,
    [string]$Duration = "30s",
    [int]$Pausa = 5
)

# Con "powershell -File", "-Pruebas d1,login" llega como un solo texto: se separa por comas.
$Pruebas = @($Pruebas | ForEach-Object { $_ -split "," } | ForEach-Object { $_.Trim() } | Where-Object { $_ })
$Vus = @($Vus | ForEach-Object { $_ -split "," } | Where-Object { $_.Trim() } | ForEach-Object { [int]$_.Trim() })

# "Continue": docker y k6 escriben progreso en stderr; los fallos se revisan con $LASTEXITCODE.
$ErrorActionPreference = "Continue"
$Perf = $PSScriptRoot
$Repo = (Resolve-Path (Join-Path $Perf "..\..")).Path
if (-not $ComposeDir) { $ComposeDir = $Repo }
if (-not $ComposeFile) {
    if (Test-Path (Join-Path $ComposeDir "docker-compose.p2.yml")) { $ComposeFile = "docker-compose.p2.yml" }
    else { $ComposeFile = "docker-compose.yml" }
}
$Override = Join-Path $Perf "sin-cache\compose.sin-cache.yml"
$TlsCompose = Join-Path $Perf "tls\compose.yml"

$K6 = (Get-Command k6 -ErrorAction SilentlyContinue).Source
if (-not $K6) { $K6 = "C:\Program Files\k6\k6.exe" }
if (-not (Test-Path $K6)) { throw "No se encontro k6. Instalelo con: winget install k6" }

$Stamp = Get-Date -Format "yyyyMMdd-HHmm"
$Out = Join-Path $Perf "resultados\$Stamp"
New-Item -ItemType Directory -Force $Out | Out-Null
$Manifest = Join-Path $Out "manifiesto.csv"
"archivo,prueba,modo,base_url,vus,rep,duracion,no_reuse,inicio,fin,time_wait_inicio,time_wait_fin,comando" | Out-File -Encoding utf8 $Manifest

function Write-Paso($msg) { Write-Host ("[{0}] {1}" -f (Get-Date -Format "HH:mm:ss"), $msg) -ForegroundColor Cyan }

# ---- Condiciones del equipo ----------------------------------------------------
# En un portatil, con bateria Windows baja el rendimiento de la CPU y, si se cierra
# la tapa, suspende el equipo a mitad de una corrida. Por eso: (1) se pide a Windows
# que no suspenda por inactividad mientras corre el script, (2) no se empieza una
# corrida con el equipo en bateria y (3) si una corrida dura mucho mas de lo debido
# (suspension), se descarta y se repite.
Add-Type -Namespace Jkr -Name Energia -MemberDefinition '[DllImport("kernel32.dll")] public static extern uint SetThreadExecutionState(uint esFlags);' -ErrorAction SilentlyContinue
[void][Jkr.Energia]::SetThreadExecutionState([uint32]"0x80000001")  # ES_CONTINUOUS | ES_SYSTEM_REQUIRED

function Test-EnBateria {
    $b = Get-CimInstance Win32_Battery -ErrorAction SilentlyContinue | Select-Object -First 1
    return ($b -and $b.BatteryStatus -eq 1)  # 1 = descargando (sin cargador)
}
function Wait-Cargador {
    $avisado = $false
    while (Test-EnBateria) {
        if (-not $avisado) { Write-Warning "El equipo esta en bateria: conecte el cargador (la corrida espera)"; $avisado = $true }
        Start-Sleep -Seconds 10
    }
}
$DurSeg = if ($Duration -match '^(\d+)s$') { [int]$Matches[1] } elseif ($Duration -match '^(\d+)m$') { 60 * [int]$Matches[1] } else { 30 }

# ---- Datos de la maquina (D5) ------------------------------------------------
$cpu = Get-CimInstance Win32_Processor | Select-Object -First 1
$os = Get-CimInstance Win32_OperatingSystem
$docker = docker info --format "{{.ServerVersion}}|{{.NCPU}}|{{.MemTotal}}|{{.KernelVersion}}"
$d = $docker -split "\|"
$k6v = (& $K6 version) -join " "
@{
    cpu              = $cpu.Name.Trim()
    nucleos          = $cpu.NumberOfCores
    hilos            = $cpu.NumberOfLogicalProcessors
    ram_gib          = [math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1GB, 1)
    so               = "$($os.Caption) (build $($os.BuildNumber))"
    docker           = "Docker Desktop, Engine $($d[0])"
    docker_cpus      = $d[1]
    docker_mem_gib   = [math]::Round([double]$d[2] / 1GB, 1)
    docker_kernel    = $d[3]
    k6               = $k6v
    base_url         = $BaseUrl
    duracion_corrida = $Duration
    fecha            = (Get-Date -Format "yyyy-MM-dd HH:mm")
} | ConvertTo-Json | Out-File -Encoding utf8 (Join-Path $Out "maquina.json")

# ---- Utilidades --------------------------------------------------------------
function Invoke-Compose([string[]]$CArgs) {
    Push-Location $ComposeDir
    try {
        & docker compose @CArgs 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "docker compose $($CArgs -join ' ') fallo" }
    } finally { Pop-Location }
}

function Invoke-K6([string]$Script, [string]$Prueba, [string]$Modo, [string]$Url, [int]$V, [int]$Rep, [hashtable]$Extra = @{}) {
    $nombre = "$Prueba-$Modo-v$V-r$Rep"
    $json = Join-Path $Out "$nombre.json"
    $envArgs = @("-e", "BASE_URL=$Url", "-e", "VUS=$V", "-e", "DURATION=$Duration", "-e", "MODO=$Modo")
    foreach ($k in $Extra.Keys) { $envArgs += @("-e", "$k=$($Extra[$k])") }
    $k6Args = @("run", "--quiet", "--summary-export", $json) + $envArgs + @((Join-Path $Perf $Script))
    $cmd = "k6 " + (($k6Args | ForEach-Object { if ($_ -match " ") { "`"$_`"" } else { $_ } }) -join " ")
    $cmd = $cmd.Replace($Repo + "\", "").Replace("\", "/")

    # docker stats a mitad de la corrida (para ver que contenedor se satura)
    $statsFile = Join-Path $Out "$nombre.stats.txt"
    $job = Start-Job -ScriptBlock {
        param($f) Start-Sleep -Seconds 15
        docker stats --no-stream --format "{{.Name}};{{.CPUPerc}};{{.MemUsage}}" | Out-File -Encoding utf8 $f
    } -ArgumentList $statsFile

    for ($intento = 1; $intento -le 3; $intento++) {
        Wait-Cargador
        $twIni = Wait-Puertos
        Write-Paso "$nombre  (TIME_WAIT inicial en el gateway: $twIni)"
        $ini = Get-Date -Format "HH:mm:ss"
        $t0 = Get-Date
        & $K6 @k6Args 2>&1 | Out-File -Encoding utf8 (Join-Path $Out "$nombre.log")
        $fin = Get-Date -Format "HH:mm:ss"
        $seg = ((Get-Date) - $t0).TotalSeconds
        # setup (registro de jugadores) + corrida + gracefulStop; mas que eso es una suspension
        if ($seg -le $DurSeg + 45 -and -not (Test-EnBateria)) { break }
        Write-Warning ("{0}: duro {1:N0} s o termino en bateria; se descarta y se repite" -f $nombre, $seg)
        Remove-Item -Force -ErrorAction SilentlyContinue $json
    }
    $twFin = Get-TimeWait
    Wait-Job $job -Timeout 60 | Out-Null; Remove-Job $job -Force
    if (-not (Test-Path $json)) { Write-Warning "k6 no genero $json (ver $nombre.log)" }
    "$nombre.json,$Prueba,$Modo,$Url,$V,$Rep,$Duration,$($Extra['NO_REUSE']),$ini,$fin,$twIni,$twFin,`"$($cmd.Replace('"',''''))`"" | Out-File -Append -Encoding utf8 $Manifest
    Start-Sleep -Seconds $Pausa
}

# El gateway abre una conexion TCP nueva por peticion hacia cada servicio
# (proxy_pass con variable, sin keepalive). Cada conexion cerrada deja el puerto
# 60 s en TIME_WAIT y el contenedor solo tiene ~28.000 puertos efimeros. Para que
# una corrida no herede los puertos ocupados por la anterior, se espera a que el
# gateway los libere antes de empezar.
function Get-TimeWait {
    $n = docker exec jokerarena-gateway sh -c "netstat -tan 2>/dev/null | grep -c TIME_WAIT"
    if ($LASTEXITCODE -ne 0 -and -not $n) { return -1 }
    return [int]($n | Select-Object -First 1)
}
function Wait-Puertos([int]$Max = 500) {
    for ($i = 0; $i -lt 60; $i++) {
        $n = Get-TimeWait
        if ($n -lt $Max) { return $n }
        Start-Sleep -Seconds 3
    }
    return (Get-TimeWait)
}

function Wait-Url([string]$Url) {
    for ($i = 0; $i -lt 30; $i++) {
        try { $r = Invoke-WebRequest -UseBasicParsing -TimeoutSec 5 $Url; if ($r.StatusCode -eq 200) { return } } catch {}
        Start-Sleep -Seconds 2
    }
    throw "No respondio $Url"
}

# Certificados locales del P3: PowerShell 5.1 no tiene -SkipCertificateCheck.
Add-Type @"
using System.Net; using System.Security.Cryptography.X509Certificates;
public class JkrTrustAll : ICertificatePolicy { public bool CheckValidationResult(ServicePoint a, X509Certificate b, WebRequest c, int d) { return true; } }
"@ -ErrorAction SilentlyContinue
[System.Net.ServicePointManager]::CertificatePolicy = New-Object JkrTrustAll
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12

Wait-Url "$BaseUrl/health"

# ---- D1: ranking con y sin cache, y salas ------------------------------------
if ($Pruebas -contains "d1") {
    Write-Paso "Sembrando datos del ranking (1.000 jugadores, 4.000 partidas)"
    # docker cp + cypher-shell -f: PowerShell 5.1 agrega BOM al pasar texto por tuberia.
    & docker cp (Join-Path $Perf "seed\sembrar_ranking.cypher") jokerarena-neo4j:/tmp/sembrar_ranking.cypher 2>&1 | Out-Null
    & docker exec jokerarena-neo4j sh -c 'cypher-shell -u neo4j -p "${NEO4J_AUTH#neo4j/}" -f /tmp/sembrar_ranking.cypher' 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "No se pudo sembrar el ranking en Neo4j" }

    # Ranking CON cache (configuracion normal del sistema)
    Invoke-Compose @("-f", $ComposeFile, "up", "-d", "--no-deps", "--wait", "jokerarena-leaderboard-stats-ms")
    Wait-Url "$BaseUrl/api/leaderboard/top?limit=10"
    foreach ($v in $Vus) { for ($r = 1; $r -le $Reps; $r++) { Invoke-K6 "ranking.js" "d1-ranking" "cache" $BaseUrl $v $r } }

    # Ranking SIN cache (override que deja al servicio sin Redis)
    try {
        Invoke-Compose @("-f", $ComposeFile, "-f", $Override, "up", "-d", "--no-deps", "--wait", "jokerarena-leaderboard-stats-ms")
        Wait-Url "$BaseUrl/api/leaderboard/top?limit=10"
        foreach ($v in $Vus) { for ($r = 1; $r -le $Reps; $r++) { Invoke-K6 "ranking.js" "d1-ranking" "sin-cache" $BaseUrl $v $r } }
    } finally {
        Write-Paso "Restaurando el Servicio de Ranking con cache"
        Invoke-Compose @("-f", $ComposeFile, "up", "-d", "--no-deps", "--wait", "jokerarena-leaderboard-stats-ms")
    }

    # Salas (ruta protegida con JWT)
    foreach ($v in $Vus) { for ($r = 1; $r -le $Reps; $r++) { Invoke-K6 "salas.js" "d1-salas" "jwt" $BaseUrl $v $r } }
}

# ---- D1: limite del login (evidencia, no capacidad) ---------------------------
if ($Pruebas -contains "login") {
    $nombre = "d1-login-limite-rate40"
    $json = Join-Path $Out "$nombre.json"
    $k6Args = @("run", "--quiet", "--summary-export", $json, "-e", "BASE_URL=$BaseUrl", "-e", "RATE=40", "-e", "DURATION=20s", (Join-Path $Perf "login_limite.js"))
    Wait-Puertos | Out-Null
    Write-Paso $nombre
    $ini = Get-Date -Format "HH:mm:ss"
    & $K6 @k6Args 2>&1 | Out-File -Encoding utf8 (Join-Path $Out "$nombre.log")
    $fin = Get-Date -Format "HH:mm:ss"
    "$nombre.json,d1-login,limite,$BaseUrl,,1,20s,,$ini,$fin,,,`"k6 run --summary-export $nombre.json -e BASE_URL=$BaseUrl -e RATE=40 -e DURATION=20s perf/http/login_limite.js`"" | Out-File -Append -Encoding utf8 $Manifest
}

# ---- D2: costo de TLS (mismo proxy en http y https) ---------------------------
if ($Pruebas -contains "d2") {
    Write-Paso "Levantando el banco de TLS (perf/http/tls)"
    if (-not (Test-Path (Join-Path $Perf "tls\certs\server.crt"))) {
        throw "Falta el certificado del banco: corra primero 'bash perf/http/tls/gen_cert.sh' (Git Bash)"
    }
    & docker compose -f $TlsCompose up -d 2>&1 | Out-Null
    try {
        Wait-Url "http://localhost:18081/health"
        Wait-Url "https://localhost:18443/health"
        $modos = [ordered]@{ "directo-http" = $BaseUrl; "proxy-http" = "http://localhost:18081"; "proxy-https" = "https://localhost:18443" }
        foreach ($v in $Vus) {
            for ($r = 1; $r -le $RepsTls; $r++) {
                # Modos intercalados en cada nivel para que la deriva del equipo afecte a todos por igual
                foreach ($m in $modos.Keys) {
                    Invoke-K6 "ranking.js" "d2-ranking" $m $modos[$m] $v $r
                    Invoke-K6 "salas.js" "d2-salas" $m $modos[$m] $v $r
                }
            }
        }
        # Peor caso: sin reutilizar conexiones (un handshake TLS por peticion)
        foreach ($v in @(10, 50)) {
            foreach ($m in @("proxy-http", "proxy-https")) {
                Invoke-K6 "ranking.js" "d2-ranking-noreuse" $m $modos[$m] $v 1 @{ NO_REUSE = "1" }
            }
        }
    } finally {
        & docker compose -f $TlsCompose down 2>&1 | Out-Null
    }
}

Write-Paso "Listo. Resultados en $Out"
Write-Host "Siguiente paso: python perf/http/resumir.py $($Out.Replace($Repo + '\', '').Replace('\','/'))"
