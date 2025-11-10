# InstallPlugin Function Documentation

## Overview
The `InstallPlugin` function installs a plugin from a release ID by downloading, extracting, configuring, and deploying both frontend and backend services to Kubernetes with optional database migration support.

## Request Structure

```json
{
  "engineKey": "my-plugin-engine",
  "releaseId": 123
}
```

### Parameters
- `engineKey` (string, required): Unique identifier for the plugin engine
- `releaseId` (int, required): Release ID to download from the plugin repository

## Manifest Structure

The plugin zip must contain a `manifest.json` file with the following structure:

```json
{
  "name": "My Awesome Plugin",
  "version": "1.0.0",
  "engine_key": "my-plugin-engine",
  "description": "A plugin that does amazing things",
  "docker": {
    "frontend": {
      "image": "myregistry/my-plugin-frontend:1.0.0"
    },
    "backend": {
      "image": "myregistry/my-plugin-backend:1.0.0",
      "migration": true
    }
  },
  "database": {
    "enabled": true,
    "name": "my_plugin_db",
    "user": "plugin_user",
    "password": "secure_password"
  },
  "storage": {
    "pvc_size": "10Gi"
  },
  "environment": {
    "LOG_LEVEL": "info",
    "FEATURE_FLAG_X": "true"
  }
}
```

### Manifest Fields

#### Root Level
- `name` (string): Human-readable plugin name
- `version` (string): Plugin version (semver recommended)
- `engine_key` (string): Must match the `engineKey` in the request
- `description` (string): Plugin description

#### Docker Section
- `frontend.image` (string): Docker image for the frontend service
- `backend.image` (string): Docker image for the backend service
- `backend.migration` (boolean): If true, runs database migrations via init container

#### Database Section (Optional)
- `enabled` (boolean): Whether to configure database environment variables
- `name` (string): Database name
- `user` (string): Database username
- `password` (string): Database password

**Note**: The function does not create the database automatically. You must ensure the database exists before deployment or implement database creation logic.

#### Storage Section (Optional)
- `pvc_size` (string): Size of PersistentVolumeClaim (default: "10Gi")

#### Environment Section (Optional)
- Key-value pairs of custom environment variables to inject into containers

## Installation Flow

1. **Validation**: Validates `engineKey` and `releaseId`
2. **Directory Setup**: Creates temporary and destination directories
3. **Download**: Downloads plugin zip from API endpoint
4. **Extraction**: Extracts zip contents
5. **Manifest Parsing**: Reads and validates manifest.json
6. **EngineKey Validation**: Ensures manifest engine_key matches request
7. **Kubernetes Namespace**: Creates/ensures plugin namespace exists
8. **PVC Creation**: Creates PersistentVolumeClaim for plugin data
9. **Artifact Copy**: Copies extracted files to destination directory
10. **Database Setup** (if enabled): Configures database environment variables and migration init container
11. **Frontend Deployment**: Creates frontend deployment and service
12. **Backend Deployment**: Creates backend deployment with optional init containers
13. **Service Creation**: Creates ClusterIP services for frontend and backend
14. **Ingress Configuration**: Adds ingress rules for routing
15. **Cleanup**: Removes temporary files

## Database Migration Support

When `database.enabled` is `true` and `backend.migration` is `true`, an init container is created that runs before the main backend container. The init container:

- Runs with the same image as the backend
- Executes: `/app/migrate` command
- On failure, attempts rollback with: `/app/migrate-rollback`
- Has database credentials in environment variables

### Required Migration Script
Your backend Docker image must include:
- `/app/migrate`: Script to run migrations
- `/app/migrate-rollback`: Script to rollback migrations on failure

Example migration script:
```bash
#!/bin/bash
set -e
echo "Running database migrations..."
# Your migration logic here (e.g., flyway, liquibase, custom SQL)
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -f /migrations/schema.sql
echo "Migrations completed successfully"
```

## API Endpoint

**POST** `/api/plugin/install`

### Request Body
```json
{
  "engineKey": "my-plugin-engine",
  "releaseId": 123
}
```

### Success Response (200 OK)
```json
{
  "message": "Plugin 'My Awesome Plugin' (v1.0.0) installed successfully!\nFrontend: http://localhost/plugins/my-plugin-engine\nBackend: http://localhost/plugins/my-plugin-engine/api\nNamespace: plugin\nDatabase: true",
  "statusCode": 200,
  "status": true,
  "data": null
}
```

### Error Response (400/500)
```json
{
  "message": "Failed to install plugin: <error details>",
  "statusCode": 500,
  "status": false,
  "data": null
}
```

## Environment Variables

Configure these environment variables for proper operation:

- `DB_HOST`: Database host (default: `postgres-service.default.svc.cluster.local`)
- `INGRESS_DOMAIN`: Domain for ingress URLs (default: `localhost`)

## Configuration Constants

Located in `plugin/constants.go`:

- `pluginNamespace`: Kubernetes namespace for plugins (default: "plugin")
- `storageClassName`: Storage class for PVCs (default: "nfs-csi-model")
- `ZipURL`: Base URL for plugin downloads
- `ZipEndpoint`: API endpoint path for downloads
- `ingressName`: Ingress resource name (default: "aistudio-ingress")

## Deployment Details

### Frontend
- **Deployment Name**: `{engineKey}-frontend`
- **Service Name**: `{engineKey}-frontend`
- **Port**: 80
- **Ingress Path**: `/plugins/{engineKey}`

### Backend
- **Deployment Name**: `{engineKey}-backend`
- **Service Name**: `{engineKey}-backend`
- **Port**: 8080
- **Ingress Path**: `/plugins/{engineKey}/api`

### Persistent Volume
- **PVC Name**: `{engineKey}-data`
- **Mount Path**: `/data` (in backend container)
- **Access Mode**: ReadWriteMany

## Error Handling

The function handles errors at each step and returns descriptive error messages:

- Missing required parameters
- Download failures
- Extraction failures
- Manifest parsing errors
- Engine key mismatch
- Kubernetes resource creation failures
- PVC creation failures
- Artifact copy failures

All errors are logged and returned to the caller with appropriate HTTP status codes.

## Cleanup

Temporary files are automatically cleaned up:
- Downloaded zip file is removed after extraction
- Extraction directory is removed after successful installation

## Example Usage

### Using curl
```bash
curl -X POST http://localhost:8080/api/plugin/install \
  -H "Content-Type: application/json" \
  -d '{
    "engineKey": "my-plugin-engine",
    "releaseId": 123
  }'
```

### Using Go
```go
import "Kubernetes-api/plugin"

req := plugin.InstallPluginRequest{
    EngineKey: "my-plugin-engine",
    ReleaseId: 123,
}

message, err := plugin.InstallPlugin(req)
if err != nil {
    log.Fatalf("Installation failed: %v", err)
}

fmt.Println(message)
```

## Troubleshooting

### Common Issues

1. **"manifest.json not found"**
   - Ensure your plugin zip contains a `manifest.json` file at the root or in a subdirectory

2. **"manifest engine_key does not match request"**
   - The `engine_key` in manifest.json must exactly match the `engineKey` in the request

3. **"failed to create PVC"**
   - Check if storage class exists: `kubectl get storageclass`
   - Verify namespace permissions

4. **"failed to create deployment"**
   - Check if images are accessible from the cluster
   - Verify RBAC permissions
   - Check node selector matches available nodes

5. **Migration init container fails**
   - Check init container logs: `kubectl logs -n plugin {engineKey}-backend-xxx -c migration`
   - Verify database connectivity from cluster
   - Ensure migration scripts exist in the image

### Debugging

View plugin resources:
```bash
# List all plugin deployments
kubectl get deployments -n plugin

# List all plugin services
kubectl get services -n plugin

# View deployment details
kubectl describe deployment {engineKey}-backend -n plugin

# View logs
kubectl logs -n plugin deployment/{engineKey}-backend

# View init container logs (if migration enabled)
kubectl logs -n plugin deployment/{engineKey}-backend -c migration
```

## Future Enhancements

- [ ] Automatic database creation
- [ ] Support for multiple init containers
- [ ] Configurable resource limits (CPU/Memory)
- [ ] Support for secrets management
- [ ] Plugin versioning and rollback
- [ ] Health check configuration
- [ ] Auto-scaling support
- [ ] Custom volume mount configurations
