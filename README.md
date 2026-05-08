# Inspector

Inspector es una herramienta de desarrollo escrita en Go para inspeccionar, depurar y enviar solicitudes HTTP, webhooks y WebSocket. Proporciona un dashboard web en tiempo real donde puedes recibir peticiones entrantes, analizar sus cabeceras y cuerpos, y también enviar solicitudes salientes con historial completo.

---

## Características

- **Recepción de HTTP / Webhooks** — Crea endpoints con slugs personalizados (`/in/:slug`) que capturan cualquier petición entrante (GET, POST, PUT, DELETE, etc.) y almacenan cabeceras, cuerpo, IP remota y más.
- **Recepción de WebSocket** — Cada endpoint también expone `ws://.../in/:slug/ws` para capturar mensajes WebSocket.
- **Dashboard en tiempo real** — Feed en vivo con Server-Sent Events (SSE): las nuevas peticiones aparecen sin recargar la página.
- **Historial de peticiones** — Listado filtrable y paginado de todas las peticiones recibidas, con vista de detalle completa.
- **Comparador de peticiones** — Vista `/requests/diff` para comparar dos capturas por método, path, headers, query y body.
- **Gestor de Endpoints** — CRUD completo: crear, editar, eliminar y limpiar el historial de cada endpoint.
- **Mock Rules** — Reglas de respuesta condicional con alcance por endpoint o global, prioridad determinística, activación/desactivación y edición en modal.
- **Exclusión por endpoint en reglas globales** — Una regla global puede excluir endpoints específicos para no aplicarse en esos slugs.
- **Enviador HTTP** — Construye y envía peticiones HTTP personalizadas (método, URL, cabeceras, cuerpo) con registro del resultado.
- **Cliente WebSocket** — Conéctate a cualquier servidor WebSocket, envía mensajes y visualiza la conversación en tiempo real.
- **Historial de envíos avanzado** — Registro de salidas con filtros por tipo, método, estado, texto y rango de fechas.
- **Redacción de datos sensibles** — Oculta valores sensibles en headers/body/query antes de persistir en base de datos.
- **Alertas de fallos salientes** — Webhook opcional cuando un envío falla o devuelve estado crítico.
- **Autenticación por sesión** — Login con formulario y cookie de sesión. Sin popups del navegador.
- **Auto-purga** — Limpieza automática de registros antiguos según `max_requests` en la configuración.
- **Sin CGO** — SQLite embebido mediante driver puro en Go (`glebarez/sqlite`). No requiere bibliotecas C externas.

---

## Stack Tecnológico

| Componente | Librería / Tecnología |
|---|---|
| Web framework | [Gin](https://github.com/gin-gonic/gin) v1.12 |
| Templates | Go HTML templates + [multitemplate](https://github.com/gin-contrib/multitemplate) |
| Base de datos | SQLite vía [GORM](https://gorm.io) + [glebarez/sqlite](https://github.com/glebarez/sqlite) |
| WebSocket | [gorilla/websocket](https://github.com/gorilla/websocket) v1.5 |
| Tiempo real | Server-Sent Events (SSE) nativos |
| Config | YAML vía [goccy/go-yaml](https://github.com/goccy/go-yaml) |
| Frontend | Tailwind CSS (CDN) + HTMX + Lucide Icons |
| Fuente | Space Grotesk (Google Fonts) |

---

## Requisitos

- Go 1.21 o superior
- No requiere CGO ni bibliotecas externas del sistema

---

## Instalación y Ejecución

### 1. Clonar / descargar el proyecto

```bash
git clone <repo-url> inspector
cd inspector
```

### 2. Instalar dependencias

```bash
go mod tidy
```

### 3. Configurar

Copia el archivo de ejemplo y edítalo con tus credenciales y preferencias:

```bash
cp config.example.yaml config.yaml
```

> **Importante:** `config.yaml` está excluido del repositorio porque contiene credenciales. Nunca lo commitees. Usa `config.example.yaml` como referencia.

Consulta la sección [Configuración](#configuración) para ver todas las opciones disponibles.

### 4. Ejecutar

```bash
# Modo desarrollo
go run main.go

# Compilar binario
go build -o inspector .
./inspector

# Con archivo de config personalizado
go run main.go mi-config.yaml
```

La aplicación estará disponible en `http://localhost:9090` (o el puerto configurado).

---

## Docker (Producción y Desarrollo)

El proyecto ya incluye dockerización completa con imagen segura `distroless` + ejecución non-root.

### Producción

1. Prepara `config.yaml` local (puedes partir de `config.example.yaml`).
2. Construye y levanta:

```bash
docker compose up -d --build
```

3. Verifica estado:

```bash
docker compose ps
docker compose logs -f inspector
```

El compose de producción usa:
- volumen persistente `inspector_data` para SQLite
- `read_only` root filesystem + `tmpfs` para endurecimiento
- `healthcheck` real contra `/readyz`
- variables seguras de producción (`INSPECTOR_ENV=production`, bloqueo de credenciales default)

Credenciales del dashboard en Docker Compose (producción):
- `INSPECTOR_AUTH_USERNAME` (default: `inspector-admin`)
- `INSPECTOR_AUTH_PASSWORD` (default: `change-this-password-now`)

Puedes sobreescribirlas antes de levantar:

```bash
export INSPECTOR_AUTH_USERNAME="admin-prod"
export INSPECTOR_AUTH_PASSWORD="tu-clave-segura"
docker compose up -d --build
```

### Desarrollo

```bash
docker compose -f docker-compose.dev.yml up -d
```

Este modo monta el código fuente y ejecuta `go run` dentro del contenedor para iteración rápida.

### Comandos Make (opcional)

```bash
make docker-build
make docker-up
make docker-down
make docker-dev-up
make docker-dev-down
```

---

## Exposición Pública (con o sin Docker)

Puedes exponer tu instancia de Inspector de forma segura usando un túnel (ngrok/localtunnel/Cloudflare), ya sea ejecutando localmente o en contenedor.

Antes de empezar, levanta la app localmente:

```bash
go run main.go
```

Por defecto se expone en `http://127.0.0.1:9090`.

### Opción 1: ngrok

1. Crea una cuenta en ngrok e instala la CLI.
2. Autentica tu cliente:

```bash
ngrok config add-authtoken TU_TOKEN
```

3. Crea el túnel HTTP hacia Inspector:

```bash
ngrok http 9090
```

4. ngrok te dará una URL pública (ejemplo):

```text
https://abcd-1234.ngrok-free.app
```

Tus endpoints quedarían así:
- HTTP: `https://abcd-1234.ngrok-free.app/in/<slug>`
- WS: `wss://abcd-1234.ngrok-free.app/in/<slug>/ws`

### Opción 2: localtunnel

1. Instala localtunnel (global o con npx):

```bash
npm install -g localtunnel
```

2. Inicia el túnel:

```bash
lt --port 9090
```

Opcionalmente puedes pedir un subdominio:

```bash
lt --port 9090 --subdomain mi-inspector-dev
```

URL de ejemplo:

```text
https://mi-inspector-dev.loca.lt
```

### Opción 3: Cloudflare Tunnel con dominio propio

Esta opción usa tu propio dominio (o subdominio) gestionado en Cloudflare.

1. Instala `cloudflared` e inicia sesión:

```bash
cloudflared tunnel login
```

2. Crea un túnel nombrado:

```bash
cloudflared tunnel create inspector
```

3. Crea el DNS route (ejemplo con `inspector.tudominio.com`):

```bash
cloudflared tunnel route dns inspector inspector.tudominio.com
```

4. Crea configuración en `~/.cloudflared/config.yml`:

```yaml
tunnel: inspector
credentials-file: C:/Users/TU_USUARIO/.cloudflared/<TUNNEL_ID>.json

ingress:
  - hostname: inspector.tudominio.com
    service: http://127.0.0.1:9090
  - service: http_status:404
```

5. Ejecuta el túnel:

```bash
cloudflared tunnel run inspector
```

Con esto podrás usar:
- HTTP: `https://inspector.tudominio.com/in/<slug>`
- WS: `wss://inspector.tudominio.com/in/<slug>/ws`

### Opción 4: Cloudflare Quick Tunnel (sin dominio)

Si no quieres configurar dominio, usa un túnel temporal:

```bash
cloudflared tunnel --url http://127.0.0.1:9090
```

Obtendrás una URL pública tipo:

```text
https://random-name.trycloudflare.com
```

Nota importante:
- Quick Tunnels son ideales para pruebas rápidas.
- La URL puede cambiar al reiniciar el comando.
- No requieren cuenta ni configuración DNS previa.

### Recomendaciones para webhooks y WebSocket

- Para webhooks externos, usa siempre la URL HTTPS que te entregue el túnel.
- Para WebSocket, cambia `ws://` por `wss://` cuando uses URL pública.
- Si pruebas firmas de webhook (GitHub/Stripe), verifica que el proveedor apunte al dominio del túnel correcto.

---

## Configuración

El archivo `config.yaml` controla todos los parámetros:

```yaml
server:
  host: "0.0.0.0"   # Interfaz de escucha (0.0.0.0 = todas las interfaces)
  port: 9090         # Puerto HTTP

auth:
  username: "admin"          # Usuario para el dashboard
  password: "inspector123"   # Contraseña para el dashboard

database:
  path: "./inspector.db"   # Ruta al archivo SQLite

settings:
  max_requests: 10000               # Máximo de peticiones almacenadas (auto-purga)
  max_request_body_bytes: 1048576   # Límite de payload entrante en /in/:slug (1MB)
  max_response_body_bytes: 2097152  # Límite de body leído por Sender (2MB)
  cleanup_interval_seconds: 30      # Frecuencia del worker de limpieza
  session_ttl_hours: 12             # TTL de sesión autenticada
  allowed_ws_origins: []            # Origins permitidos para WS (vacío = same-origin + clientes sin Origin)
  redaction_enabled: true           # Habilita redacción de campos sensibles al guardar logs
  redaction_headers:                # Headers a redactar (case-insensitive)
    - authorization
    - cookie
    - x-api-key
    - x-auth-token
  redaction_fields:                 # Claves sensibles en body/query JSON o texto
    - password
    - token
    - secret
    - api_key
  alert_webhook_url: ""            # URL opcional para recibir alertas de envíos críticos
  alert_min_sent_status: 500        # Umbral mínimo de status HTTP para alertar
  alert_on_sent_error: true         # Alertar también cuando hay error de red/timeout
```

### Filtros avanzados

- `GET /requests` soporta: `type`, `endpoint`, `method`, `q`, `from`, `to`, `page`
- `GET /send/history` soporta: `type`, `method`, `status` (ej: `200` o `error`), `q`, `from`, `to`, `page`

### Redacción y alertas

- La redacción se aplica al persistir eventos entrantes y salientes.
- Los campos definidos en `redaction_headers` y `redaction_fields` se guardan como `***REDACTED***`.
- Si `alert_webhook_url` está configurado, Inspector envía un POST JSON al webhook cuando:
  - hay error saliente (si `alert_on_sent_error: true`), o
  - `response_status >= alert_min_sent_status`.

Se puede pasar un archivo de configuración alternativo como argumento:

```bash
./inspector config.production.yaml
```

### Overrides por variables de entorno

Además del YAML, puedes sobreescribir configuración con variables de entorno:

- `INSPECTOR_SERVER_HOST`
- `INSPECTOR_SERVER_PORT`
- `INSPECTOR_AUTH_USERNAME`
- `INSPECTOR_AUTH_PASSWORD`
- `INSPECTOR_DATABASE_PATH`
- `INSPECTOR_SETTINGS_MAX_REQUESTS`
- `INSPECTOR_SETTINGS_MAX_REQUEST_BODY_BYTES`
- `INSPECTOR_SETTINGS_MAX_RESPONSE_BODY_BYTES`
- `INSPECTOR_SETTINGS_CLEANUP_INTERVAL_SECONDS`
- `INSPECTOR_SETTINGS_SESSION_TTL_HOURS`
- `INSPECTOR_SETTINGS_ALLOWED_WS_ORIGINS` (CSV)
- `INSPECTOR_SETTINGS_REDACTION_ENABLED`
- `INSPECTOR_SETTINGS_REDACTION_HEADERS` (CSV)
- `INSPECTOR_SETTINGS_REDACTION_FIELDS` (CSV)
- `INSPECTOR_SETTINGS_ALERT_WEBHOOK_URL`
- `INSPECTOR_SETTINGS_ALERT_MIN_SENT_STATUS`
- `INSPECTOR_SETTINGS_ALERT_ON_SENT_ERROR`

Variables operativas adicionales:
- `INSPECTOR_ENV` (ej. `production`)
- `INSPECTOR_ALLOW_DEFAULT_AUTH` (`1` solo si deseas permitir credenciales default en producción)

---

## Uso

### Login

Accede a `http://localhost:9090/login` con las credenciales configuradas. Se crea una cookie de sesión `inspector_session` que autentica todas las páginas del dashboard.

### Crear un Endpoint

1. Ve a **Endpoints** en el menú lateral.
2. Rellena el nombre, slug y descripción opcional.
3. Haz clic en **Create Endpoint**.

El endpoint quedará disponible en:
- HTTP: `http://localhost:9090/in/<slug>`
- WebSocket: `ws://localhost:9090/in/<slug>/ws`

### Recibir peticiones (webhook)

Configura cualquier servicio externo (GitHub, Stripe, Shopify, etc.) para que envíe webhooks a:

```
http://<tu-ip>:9090/in/<slug>
```

Inspector detecta automáticamente si es un webhook comprobando cabeceras comunes (`X-GitHub-Event`, `X-Hub-Signature`, `Stripe-Signature`, etc.).

### Prueba manual con cURL

```bash
# POST con JSON
curl -X POST http://localhost:9090/in/mi-endpoint \
  -H "Content-Type: application/json" \
  -d '{"evento": "pago", "importe": 49.99}'

# GET con parámetros
curl "http://localhost:9090/in/mi-endpoint?id=123&estado=activo"

# Con cabeceras personalizadas
curl -X POST http://localhost:9090/in/mi-endpoint \
  -H "X-GitHub-Event: push" \
  -H "Content-Type: application/json" \
  -d '{"ref": "refs/heads/main"}'
```

### Enviar peticiones HTTP

1. Ve a **Send Request** en el menú.
2. Elige método, URL, cabeceras y cuerpo.
3. Haz clic en **Send** — el resultado aparece inmediatamente.

### Mocking de endpoints (Mock Rules)

Inspector soporta mocking avanzado para simular comportamientos complejos sin tocar tu backend real.

#### Dónde se administra

- **Pantalla central**: `/mocks` para crear/editar/eliminar reglas globales y por endpoint.
- **Pantalla Endpoints**: muestra reglas del endpoint en modo resumen con acciones rápidas (activar/desactivar, editar en modal, eliminar).

#### Alcances disponibles

- **Endpoint**: la regla solo aplica al endpoint seleccionado.
- **Global**: aplica a todos los endpoints, excepto los que marques en **Excluir endpoints**.

#### Qué puede evaluar una regla

- Método HTTP (`ANY`, `GET`, `POST`, `PUT`, `PATCH`, `DELETE`).
- Path (`any`, `exact`, `prefix`, `regex`).
- Query params (`any`, `contains`, `exact`).
- Headers (`any`, `contains`, `exact`).
- Body (`any`, `contains`, `exact`, `regex`, `json`).

#### Qué puede responder una regla

- Status HTTP.
- Headers de respuesta (JSON).
- Body de respuesta (JSON/texto).
- Delay en milisegundos para simular latencia.
- Estado activa/inactiva.

#### Orden de evaluación (precedencia)

1. `Priority ASC` (menor número se evalúa primero).
2. Si hay empate de prioridad: una regla de alcance **endpoint** gana sobre una **global**.
3. Si persiste el empate: `ID ASC`.
4. Si una regla global excluye el endpoint actual, se omite esa regla.
5. Si ninguna regla hace match, se usa la respuesta estática del endpoint (fallback).

#### Extras de operación

- `HitCount` por regla para ver cuántas veces fue usada.
- Toggle de activación sin borrar la regla.
- Cambios de reglas emiten evento en tiempo real `mock_changed`.

### Cliente WebSocket

1. Ve a **WS Client** en el menú.
2. Introduce la URL del servidor WebSocket (`ws://...`).
3. Conéctate, envía mensajes y visualiza la conversación.

---

## Rutas de la API

### Rutas públicas (sin autenticación)

| Método | Ruta | Descripción |
|--------|------|-------------|
| `ANY` | `/in/:slug` | Recibe cualquier petición HTTP en el endpoint |
| `GET` | `/in/:slug/ws` | Upgrade a WebSocket en el endpoint |
| `GET` | `/healthz` | Liveness probe del servicio |
| `GET` | `/readyz` | Readiness probe (incluye chequeo de DB) |
| `GET` | `/login` | Página de login |
| `POST` | `/login` | Procesa credenciales y crea sesión |
| `GET` | `/logout` | Cierra sesión y elimina cookie |

### Rutas autenticadas (requieren cookie `inspector_session`)

#### Dashboard y Peticiones

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/dashboard` | Dashboard principal con endpoints y live feed |
| `GET` | `/requests` | Listado de peticiones recibidas (filtros: `type`, `endpoint`, `method`, `q`, `from`, `to`, `page`) |
| `GET` | `/requests/diff` | Compara dos peticiones capturadas (`left`, `right`, `endpoint`) |
| `GET` | `/requests/:id` | Detalle completo de una petición |

#### Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/endpoints` | Listado y gestión de endpoints |
| `POST` | `/endpoints` | Crear nuevo endpoint |
| `PUT` | `/endpoints/:id` | Actualizar endpoint |
| `POST` | `/endpoints/:id` | Actualizar endpoint (fallback para formularios HTML) |
| `DELETE` | `/endpoints/:id` | Eliminar endpoint y sus registros |
| `POST` | `/endpoints/:id/clear` | Limpiar historial de peticiones del endpoint |

#### Mock Rules por endpoint

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/endpoints/:id/mocks` | Lista reglas mock asociadas al endpoint |
| `POST` | `/endpoints/:id/mocks` | Crea regla mock para ese endpoint |
| `PUT` | `/endpoints/:id/mocks/:mockId` | Actualiza regla mock del endpoint |
| `POST` | `/endpoints/:id/mocks/:mockId` | Actualiza regla mock (fallback formulario HTML) |
| `DELETE` | `/endpoints/:id/mocks/:mockId` | Elimina regla mock del endpoint |
| `POST` | `/endpoints/:id/mocks/:mockId/toggle` | Activa/desactiva regla mock del endpoint |

#### Mock Rules (gestión central)

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/mocks` | Página central de administración de Mock Rules |
| `GET` | `/mocks/global` | Lista JSON de reglas globales |
| `POST` | `/mocks` | Crea regla global o crea reglas por endpoint (multi-select) |
| `PUT` | `/mocks/:mockId` | Actualiza regla mock (global o endpoint) |
| `POST` | `/mocks/:mockId` | Actualiza regla mock (fallback formulario HTML) |
| `DELETE` | `/mocks/:mockId` | Elimina regla mock |
| `POST` | `/mocks/:mockId/toggle` | Activa/desactiva regla mock |

#### Enviador

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/send` | Formulario para enviar peticiones HTTP |
| `POST` | `/send/http` | Ejecuta el envío HTTP |
| `GET` | `/send/history` | Historial de peticiones enviadas (filtros: `type`, `method`, `status`, `q`, `from`, `to`, `page`) |
| `GET` | `/send/history/:id` | Detalle de una petición enviada |
| `GET` | `/send/ws-client` | Página del cliente WebSocket |
| `GET` | `/send/ws-proxy` | Proxy WebSocket para el cliente (upgrade) |

#### Tiempo real

| Método | Ruta | Descripción |
|--------|------|-------------|
| `GET` | `/events` | Stream SSE para actualizaciones en tiempo real |
| `GET` | `/events/ws` | Stream de eventos en WebSocket (fallback recomendado tras túneles) |
| `GET` | `/events/poll` | Polling JSON de eventos (compatibilidad) |

---

## Eventos SSE

El endpoint `/events` emite eventos en tiempo real que el frontend consume para actualizar la UI sin recargar:

### `new_request`
Emitido cuando llega una nueva petición a cualquier endpoint.

```json
{
  "type": "new_request",
  "data": {
    "id": 42,
    "endpoint_slug": "mi-endpoint",
    "type": "webhook",
    "method": "POST",
    "path": "/in/mi-endpoint",
    "remote_addr": "192.168.1.1:54321",
    "size_bytes": 256,
    "created_at": "2026-05-05T10:30:00Z"
  }
}
```

### `new_sent_request`
Emitido al completar el envío de una petición HTTP saliente.

```json
{
  "type": "new_sent_request",
  "data": {
    "id": 15,
    "type": "http",
    "method": "POST",
    "url": "https://api.ejemplo.com/webhook",
    "status": 200,
    "duration_ms": 143,
    "error": "",
    "created_at": "2026-05-05T10:31:00Z"
  }
}
```

### `endpoint_changed`
Emitido al crear, actualizar, eliminar o limpiar un endpoint.

```json
{
  "type": "endpoint_changed",
  "data": {
    "action": "created",
    "id": 7
  }
}
```

Acciones posibles: `created`, `updated`, `deleted`, `cleared`.

### `mock_changed`
Emitido al crear, actualizar, eliminar o activar/desactivar una Mock Rule.

```json
{
  "type": "mock_changed",
  "data": {
    "action": "updated",
    "id": 12,
    "scope": "global",
    "endpoint_id": null,
    "slug": null,
    "is_active": true
  }
}
```

Acciones posibles: `created`, `updated`, `deleted`, `toggled`.

---

## Estructura del Proyecto

```
inspector/
├── main.go                     # Punto de entrada, router, renderer de templates
├── config.yaml                 # Configuración de la aplicación
├── go.mod / go.sum             # Módulo Go y dependencias
├── inspector.db                # Base de datos SQLite (generada en ejecución)
│
├── internal/
│   ├── config/
│   │   └── config.go           # Carga y estructura de config.yaml
│   ├── models/
│   │   ├── endpoint.go         # Modelo Endpoint (GORM)
│   │   ├── mock_rule.go        # Modelo MockRule (scope, exclusiones, prioridad, matchers)
│   │   ├── request_log.go      # Modelo RequestLog (GORM)
│   │   └── sent_request.go     # Modelo SentRequest (GORM)
│   ├── storage/
│   │   └── db.go               # Inicialización GORM, AutoMigrate, Cleanup
│   ├── broadcaster/
│   │   └── hub.go              # Hub SSE: Subscribe/Unsubscribe/Broadcast
│   ├── middleware/
│   │   └── auth.go             # Middleware de autenticación por cookie
│   └── handlers/
│       ├── auth.go             # Login, logout, gestión de sesión
│       ├── receiver.go         # Recepción HTTP y WebSocket entrante
│       ├── dashboard.go        # Dashboard, listado y detalle de peticiones
│       ├── endpoints.go        # CRUD de endpoints
│       ├── mocks.go            # CRUD/toggle de Mock Rules (endpoint + global)
│       ├── mock_matcher.go     # Resolución de reglas mock y precedencia
│       ├── sender.go           # Envío HTTP, proxy WS, historial
│       └── sse.go              # Stream de Server-Sent Events
│
└── web/
    └── templates/
        ├── layout.html         # Layout base: sidebar, nav, modal system
        ├── login.html          # Página de login
        ├── dashboard.html      # Dashboard principal
        ├── endpoints.html      # Gestión de endpoints
        ├── mocks.html          # Gestión central de Mock Rules
        ├── requests.html       # Listado de peticiones recibidas
        ├── request_detail.html # Detalle de petición recibida
        ├── sender.html         # Formulario de envío HTTP
        ├── sent_history.html   # Historial de envíos
        ├── sent_detail.html    # Detalle de envío
        └── ws_client.html      # Cliente WebSocket
```

---

## Notas Técnicas

- **Sin recarga de página**: El dashboard usa SSE + JavaScript vanilla para insertar nuevas filas en tablas y tarjetas en el feed en tiempo real.
- **Aislamiento de templates**: Se usa `gin-contrib/multitemplate` para que cada página tenga su propio `*template.Template` aislado, evitando colisiones en los bloques `{{define "content"}}`.
- **Detección de webhooks**: El receiver comprueba automáticamente la presencia de cabeceras estándar de webhook (`X-GitHub-Event`, `X-Gitlab-Event`, `Stripe-Signature`, `X-Hub-Signature`, `X-Shopify-Hmac-Sha256`, etc.).
- **Modales nativos**: El sistema de alertas y confirmaciones usa modales HTML/CSS propios en lugar de `alert()` y `confirm()` del navegador, accesibles globalmente con `window.appAlert(msg)` y `window.appConfirm(msg, callback)`.

---

## Licencia

MIT
