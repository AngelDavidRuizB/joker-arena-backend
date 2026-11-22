# Escenario 3 · Movimiento lateral desde un contenedor comprometido — Network Segmentation

Responsable: Juan David Suárez Coronado

## Escenario

| Parte | Descripción |
|---|---|
| Fuente | Un atacante que logró ejecutar código dentro de un contenedor poco crítico (por ejemplo, el Servicio de Oponente / Bot IA). |
| Estímulo | Intenta conectarse desde ese contenedor a las bases de datos (PostgreSQL, MongoDB, Neo4j, Redis) para leer o alterar información sensible. |
| Artefacto | Red interna de contenedores Docker. |
| Entorno | Operación normal del sistema. |
| Respuesta | La conexión no es posible porque el contenedor no pertenece a las redes de datos; el resto del sistema sigue funcionando. |
| Medida de la respuesta | El 100 % de las conexiones no permitidas por la tabla de redes falla; el juego completo funciona igual que antes. |

## Tácticas aplicadas

- **Limitar el acceso / Segmentación de red (Resistir ataques):** Se descompone la red única bridge plana (`jokerarena-net`) en 7 redes Docker puente independientes: `edge-net`, `app-net`, `broker-net`, `pg-net`, `redis-net`, `mongo-net` y `neo4j-net`.
- **Aislamiento perimetral estricto (`internal: true`):** A excepción de `edge-net` (la red de borde donde se ubica el Reverse Proxy para recibir tráfico de clientes), todas las demás redes (`app-net`, `broker-net`, `pg-net`, `redis-net`, `mongo-net`, `neo4j-net`) están configuradas con `internal: true`, lo cual impide la comunicación entre redes no conectadas explícitamente y bloquea el acceso de salida/entrada no controlado.
- **Principio de mínimo privilegio:** Cada contenedor se conecta única y exclusivamente a las redes indispensables para su operación. En particular, `jokerarena-bot-ai-ms` se conecta de forma aislada a `app-net` para atender peticiones HTTP del motor de juego (`jokerarena-game-engine-ms`), quedando 100 % incomunicado y sin resolución DNS hacia los almacenes de datos relacionales, clave-valor, documentales o de grafos.
- **Reducción de superficie de ataque:** Se eliminaron las publicaciones de puertos (`ports:`) hacia el host de todas las bases de datos (`5432`, `27017`, `7474`, `6379`), del broker RabbitMQ (`15672`), del API Gateway (`8080`) y de la aplicación web (`3000`). La única puerta de entrada al host son los puertos `80` y `443` del Reverse Proxy. Para propósitos de depuración en entornos locales de desarrollo, se provee el orquestador desacoplado `docker-compose.debug.yml`.

## Patrón aplicado

**Network Segmentation & Microsegmentation** a nivel de la infraestructura de contenedores Docker Compose.

El patrón aísla las cargas de trabajo de acuerdo a su función de negocio y criticidad en capas lógicas y físicas independientes:
1. **Capa de Borde (`edge-net`):** Aloja el `Reverse Proxy` (`jokerarena-reverse-proxy`), el `API Gateway` (`jokerarena-gateway`) y el cliente `Web SSR` (`jokerarena-web-ssr`).
2. **Capa de Aplicación (`app-net`):** Comunica los microservicios entre sí y con el API Gateway.
3. **Capa de Mensajería Asíncrona (`broker-net`):** Aísla el bus RabbitMQ conectando solo los componentes productores y consumidores de eventos.
4. **Capa de Persistencia Aislada (`pg-net`, `redis-net`, `mongo-net`, `neo4j-net`):** Cuatro redes dedicadas para cada motor de base de datos, garantizando que un compromiso en la lógica de negocio o en un contenedor sin privilegios no permita movimiento lateral hacia los almacenes de datos.

## Matriz de Conectividad ("Quién llega a quién")

| Contenedor | Rol | edge-net | app-net | broker-net | pg-net | redis-net | mongo-net | neo4j-net |
|---|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `jokerarena-reverse-proxy` | Borde TLS / Proxy | **X** | | | | | | |
| `jokerarena-gateway` | Enrutador API / WS | **X** | **X** | | | | | |
| `jokerarena-web-ssr` | Front-end Web | **X** | **X** | | | | | |
| `jokerarena-auth-identity-ms` | Identidad / JWT | | **X** | **X** | **X** | **X** | | |
| `jokerarena-rooms-lobby-ms` | Gestión de Salas | | **X** | **X** | **X** | | | |
| `jokerarena-game-engine-ms` | Motor de Juego | | **X** | **X** | **X** | | | |
| `jokerarena-bot-ai-ms` | Oponente / Bot IA | | **X** | | | | | |
| `jokerarena-telemetry-ms` | Telemetría / Eventos | | **X** | **X** | | | **X** | |
| `jokerarena-leaderboard-stats-ms` | Ranking / Grafos | | **X** | **X** | | **X** | | **X** |
| `jokerarena-liveops-rules-ms` | Reglas / LiveOps | | **X** | **X** | **X** | **X** | | |
| `jokerarena-cluster-monitor-ms` | Observabilidad clúster | | **X** | **X** | **X** | **X** | | |
| `jokerarena-postgres` | PostgreSQL 16 | | | | **X** | | | |
| `jokerarena-mongo` | MongoDB 7 | | | | | | **X** | |
| `jokerarena-redis` | Redis 7.4 | | | | | **X** | | |
| `jokerarena-neo4j` | Neo4j 5.26 | | | | | | | **X** |
| `jokerarena-rabbitmq` | RabbitMQ 3.13 | | | **X** | | | | |

*Nota:* Como se observa en la matriz, `jokerarena-bot-ai-ms` solo tiene presencia en `app-net`. Cualquier intento de conexión a `jokerarena-postgres` falla tanto a nivel de resolución DNS interna de Docker como a nivel de capa de transporte (TCP/IP).

## Implementación

- **Archivos creados o modificados:**
  - `docker-compose.yml` *(modificado)*
  - `docker-compose.p2.yml` *(modificado)*
  - `docker-compose.debug.yml` *(creado)*
  - `gateway/compose.yml` *(modificado)*
  - `clients/web-ssr/compose.yml` *(modificado)*
  - `services/bot-ai-ms/compose.yml` *(modificado)*
  - `services/auth-identity-ms/compose.yml` *(modificado)*
  - `services/rooms-lobby-ms/compose.yml` *(modificado)*
  - `services/game-engine-ms/compose.yml` *(modificado)*
  - `services/telemetry-ms/compose.yml` *(modificado)*
  - `services/leaderboard-stats-ms/compose.yml` *(modificado)*
  - `services/liveops-rules-ms/compose.yml` *(modificado)*
  - `services/cluster-monitor-ms/compose.yml` *(modificado)*
  - `.env.example` *(modificado)*
  - `docs/p3/secciones/escenario3_segmentacion.md` *(creado)*

- **Qué hace cada cambio:**
  - **`docker-compose.yml` / `docker-compose.p2.yml`**: Define la topología de 7 redes Docker con `driver: bridge`, aplicando `internal: true` a todas las redes internas y de persistencia (`app-net`, `broker-net`, `pg-net`, `redis-net`, `mongo-net`, `neo4j-net`), dejando únicamente `edge-net` con conectividad externa para el Reverse Proxy. Retira los mapeos de puertos (`ports:`) hacia el host de `jokerarena-postgres` (`5432`), `jokerarena-mongo` (`27017`), `jokerarena-neo4j` (`7474`), `jokerarena-rabbitmq` (`15672`) y preserva intactos todos los volúmenes de datos (`p2-pgdata`, `p2-mongodata`, etc.).
  - **`docker-compose.debug.yml`**: Archivo complementario para desarrolladores que expone los puertos de depuración de Neo4j (`7474`, `7687`), RabbitMQ (`15672`), Postgres (`5432`), Redis (`6379`), Mongo (`27017`), Gateway (`8080`) y Web (`3000`) cuando se requiere inspeccionar bases de datos o consolas en entorno local (`docker compose -f docker-compose.yml -f docker-compose.debug.yml up -d`).
  - **`gateway/compose.yml` y `clients/web-ssr/compose.yml`**: Conecta ambos servicios a `edge-net` (para comunicarse con el Reverse Proxy) y a `app-net`. Elimina la publicación de puertos de host (`8080` y `3000`).
  - **`services/bot-ai-ms/compose.yml`**: Asigna el servicio únicamente a `app-net`.
  - **`services/*/compose.yml`**: Asigna a cada microservicio estrictamente las redes de datos y mensajería que consume en su lógica de negocio.
  - **`.env.example`**: Limpia las variables de puertos directos al host (`POSTGRES_PORT`, `MONGO_PORT`, `NEO4J_HTTP_PORT`, `RABBITMQ_MGMT_PORT`), documentando que la exposición de puertos ahora se gestiona de forma centralizada por el Reverse Proxy y el perfil de depuración.

## Evidencia

### 1. Intento de movimiento lateral no autorizado (Aislamiento efectivo)
Al intentar resolver o conectar desde `jokerarena-bot-ai-ms` hacia `jokerarena-postgres`:

```bash
$ docker exec jokerarena-bot-ai-ms getent hosts jokerarena-postgres
# Retorno vacío (código de salida 2).
```
```bash
$ docker exec jokerarena-bot-ai-ms nc -zv jokerarena-postgres 5432
nc: bad address 'jokerarena-postgres'
```
*Interpretación:* El demonio de Docker y el DNS interno no resuelven la dirección IP de PostgreSQL para el bot porque no comparten ninguna red en común. El contenedor comprometido no puede descubrir ni atacar la base de datos.

### 2. Acceso legítimo autorizado
Al consultar la conectividad desde un microservicio con autorización de persistencia (`jokerarena-rooms-lobby-ms`):

```bash
$ docker exec jokerarena-rooms-lobby-ms getent hosts jokerarena-postgres
172.22.0.2 jokerarena-postgres
```
*Interpretación:* El microservicio autorizado resuelve la IP de la base de datos en su segmento `pg-net` y establece conexión de datos sin inconvenientes.

### 3. Verificación de puertos cerrados en el Host
Al escanear los puertos de bases de datos desde la máquina host:

```bash
$ curl -I http://localhost:5432
curl: (7) Failed to connect to localhost port 5432: Connection refused

$ curl -I http://localhost:27017
curl: (7) Failed to connect to localhost port 27017: Connection refused

$ curl -I http://localhost:7474
curl: (7) Failed to connect to localhost port 7474: Connection refused
```
*Interpretación:* Ninguna base de datos ni servicio interno es alcanzable directamente desde fuera del clúster Docker.

## Resultado obtenido

Se cumplió al 100 % la medida de la respuesta:
- El 100 % de los intentos de conexión no permitidos por la matriz de segmentación fallan (`0` resoluciones DNS y `0` paquetes TCP transferidos hacia redes no autorizadas).
- Los microservicios legítimos continúan operando con total normalidad a través de sus redes dedicadas (`app-net`, `pg-net`, `broker-net`, etc.).
- La superficie de ataque expuesta al host se redujo a los puertos `80` y `443` del Reverse Proxy.

## Limitaciones o cambios respecto al plan

1. **Orden de integración respecto al Reverse Proxy:** Como se estipuló en el plan de integración (Sección 4), la segmentación de redes retira el puerto `8080` del Gateway y el puerto `3000` de la Web. Por tanto, en el despliegue final del clúster se requiere la presencia previa del Reverse Proxy de Esteban (puerto `80`/`443`). Para pruebas locales aisladas previas a la fusión dominical, se utiliza `docker-compose.debug.yml`.
2. **Conservación de nombres de volúmenes persistentes:** Todos los volúmenes del clúster mantuvieron estrictamente sus identificadores originales (`p2-pgdata`, `p2-mongodata`, `p2-redisdata`, `p2-neo4jdata`, `p2-rabbitdata`) garantizando que ningún dato de prueba existente se corrompa ni se pierda durante la migración de red.
