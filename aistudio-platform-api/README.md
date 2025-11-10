# Fusemachines AI Studio Platform API

Fusemachines AI Studio Platform offers a REST interface for various functionalities, including pod and resource management, Jupyter notebook administration, model registry management, and numerous other configurations.

## Run the applications 

Run for dev with watch

```sh
make run.dev
```
for the first time you need to install gow to watch files
`go install github.com/mitranim/gow@latest`

Run the following command:

```sh
make run
```
## Things to generate swagger api doc

Inorder to Generate the Swagger Docs install swag in the project root director

```sh
go install github.com/swaggo/swag/cmd/swag@latest
swag init
```

Make sure you export in your path to run `swag init`:
```sh
export PATH=${PATH}:`go env GOPATH`/bin
```

## Plugin Installation API

The platform exposes transactional endpoints for installing and managing AI Studio plugins:

- `POST /api/plugin/install/{identifier}` – install a plugin bundle. The request accepts a `PluginInstallRequest` body containing the bundle `zipUrl`, `engineKey`, `releaseId`, optional backend callbacks, and routing preferences.
- `PUT /api/plugin/install/{identifier}` – update an existing plugin with the same payload contract.
- `DELETE /api/plugin/install/{identifier}` – rollback an installed plugin. Optionally pass `routePath` in the JSON payload or query string to remove ingress rules.

During installation the API validates the plugin ZIP and `manifest.yaml`, writes or updates `deployment.yaml`, provisions database credentials when requested, copies the plugin logo, applies Kubernetes resources, emits backend callbacks, and persists state so that the whole operation can be rolled back on failure.

Set the `KUBECONFIG` environment variable when running outside the cluster to point to the desired Kubernetes configuration file. Otherwise the installer will fall back to `$HOME/.kube/config`.
