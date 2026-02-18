# CapyFlow Backend API

API RESTful y motor de ejecución de flujos de trabajo construido en Go.

## 🚀 Inicio Rápido

### Prerrequisitos
- Go 1.21 o superior
- PostgreSQL 16 (o usar Docker Compose)
- Make (opcional)

### Instalación

```bash
# Instalar dependencias
go mod download

# Ejecutar migraciones y seeds
# Esto se hace automáticamente al iniciar la aplicación

# Iniciar el servidor
make run
# O alternativamente:
go run main.go
```

El servidor estará corriendo en `http://localhost:8080`

## 📦 Comandos Make

- `make run` - Ejecuta el servidor
- `make build` - Compila el binario
- `make clean` - Limpia archivos compilados

## 🛠️ Stack Tecnológico

- **Go 1.21+** - Lenguaje de programación
- **PostgreSQL** - Base de datos
- **GORM** - ORM
- **Gorilla WebSocket** - WebSocket
- **JWT** - Autenticación

## 🎨 Características

- ✅ API RESTful completa
- ✅ Autenticación JWT
- ✅ Motor de ejecución de flujos
- ✅ WebSocket para actualizaciones en tiempo real
- ✅ Sistema de webhooks
- ✅ Validación de nodos
- ✅ Gestión de errores
- ✅ CORS configurado

## 📁 Estructura del Proyecto

```
Apiflow/
├── config/           # Configuración de la aplicación
├── database/         # Conexión, migraciones y seeds
├── handlers/         # HTTP handlers (controllers)
├── middleware/       # Middleware (auth, cors, etc.)
├── models/           # Modelos de base de datos
├── nodes/            # Implementación de tipos de nodos
├── routes/           # Definición de rutas
├── services/         # Lógica de negocio
├── validators/       # Validadores de datos
├── websocket/        # WebSocket hub y clientes
├── main.go           # Punto de entrada
└── Makefile          # Comandos de compilación
```

## 🔧 Configuración

### Variables de Entorno

Puedes configurar las siguientes variables:

```bash
PORT=8080
DATABASE_DSN=host=localhost user=postgres password=postgres dbname=capyflow port=5432 sslmode=disable
ENVIROMENT=development
```

Si no se establecen, se usarán los valores por defecto.

## 📝 API Endpoints

### Autenticación
- `POST /api/register` - Registrar nuevo usuario
- `POST /api/login` - Iniciar sesión

### Flujos
- `GET /api/flows` - Listar todos los flujos
- `GET /api/flows/:id` - Obtener un flujo específico
- `POST /api/flows` - Crear nuevo flujo
- `PUT /api/flows/:id` - Actualizar flujo
- `DELETE /api/flows/:id` - Eliminar flujo
- `POST /api/flows/:id/execute` - Ejecutar flujo

### Nodos
- `GET /api/flows/:flowId/nodes` - Listar nodos de un flujo
- `POST /api/flows/:flowId/nodes` - Crear nodo
- `PUT /api/flows/:flowId/nodes/:id` - Actualizar nodo
- `DELETE /api/flows/:flowId/nodes/:id` - Eliminar nodo

### Tipos de Nodos
- `GET /api/node-types` - Listar tipos de nodos disponibles
- `GET /api/node-types/:id` - Obtener tipo de nodo específico

### Ejecuciones
- `GET /api/executions` - Listar ejecuciones
- `GET /api/executions/:id` - Obtener ejecución específica
- `GET /api/flows/:flowId/executions` - Listar ejecuciones de un flujo

### Webhooks
- `GET /api/webhooks/flows/:flowId/url` - Obtener URL de webhook
- `POST /api/webhooks/flows/:flowId/trigger` - Activar webhook (testing)
- `POST /api/webhook/:flowId` - Endpoint público de webhook

### WebSocket
- `GET /api/ws` - Conectar WebSocket (requiere ticket)
- `GET /api/ws/ticket` - Obtener ticket de WebSocket

## 🔐 Autenticación

Todos los endpoints (excepto `/register`, `/login` y `/webhook/:flowId`) requieren un token JWT en el header:

```
Authorization: Bearer <token>
```

## 🌐 WebSocket

Para conectarte al WebSocket:

1. Obtén un ticket: `GET /api/ws/ticket`
2. Conecta usando el ticket: `ws://localhost:8080/api/ws?ticket=<ticket>`

Los mensajes de WebSocket incluyen:
- Actualizaciones de ejecución de nodos
- Estado de flujos
- Notificaciones en tiempo real

## 🎯 Tipos de Nodos Implementados

### Triggers
- **manual-trigger** - Inicio manual
- **webhook-trigger** - Inicio por webhook HTTP

### Logic
- **if-condition** - Condicionales
- **if-condition-v2** - Condicionales mejorados
- **loop** - Bucles

### Data
- **set-data** - Establecer datos
- **transform-data** - Transformar datos
- **json-parser** - Parsear JSON

### I/O
- **http-request** - Peticiones HTTP
- **log** - Logging

### Timing
- **delay** - Retraso temporal

## 🚀 Desarrollo

### Agregar un Nuevo Tipo de Nodo

1. Crear archivo en `nodes/` (ej: `mi_nodo.go`)
2. Implementar la interfaz `NodeHandler`:

```go
type MiNodo struct{}

func (n *MiNodo) Execute(ctx *ExecutionContext) error {
    // Implementación
    return nil
}

func (n *MiNodo) Validate(params map[string]interface{}) error {
    // Validación
    return nil
}
```

3. Registrar en `nodes/registry.go`:

```go
func init() {
    RegisterNode("mi-nodo", &MiNodo{})
}
```

4. Agregar seed en `database/seed_node_types.go`

### Ejecutar Migraciones

Las migraciones se ejecutan automáticamente al iniciar la aplicación. Para forzar migraciones:

```go
// En database/database.go
db.AutoMigrate(&models.User{}, &models.Flow{}, ...)
```

## 🐛 Solución de Problemas

### Error de conexión a base de datos
- Verifica que PostgreSQL esté corriendo
- Comprueba las credenciales en `DATABASE_DSN`
- Usa Docker Compose: `cd ../capyBD && docker compose up -d`

### Puerto en uso
- Cambia el puerto con la variable `PORT`
- Verifica que no haya otra instancia corriendo

### WebSocket no funciona
- Verifica que el ticket sea válido (expira en 30 segundos)
- Comprueba la URL de WebSocket
- Revisa los logs del servidor

## 📊 Base de Datos

### Esquema Principal

- **users** - Usuarios del sistema
- **flows** - Flujos de trabajo
- **nodes** - Nodos de los flujos
- **edges** - Conexiones entre nodos
- **node_types** - Tipos de nodos disponibles
- **executions** - Historial de ejecuciones

### Reiniciar Base de Datos

```bash
cd ../capyBD
docker compose down -v
docker compose up -d
cd ../Apiflow
go run main.go  # Las migraciones se ejecutarán automáticamente
```

## 🤝 Contribución

Este es un proyecto de titulación. Para contribuir:
1. Fork el repositorio
2. Crea una rama para tu feature
3. Commit tus cambios con mensajes descriptivos
4. Push a la rama
5. Abre un Pull Request

## 📄 Licencia

Este proyecto es parte de un trabajo de titulación.

---

**Versión**: 1.0.0  
**Última actualización**: Febrero 2026
