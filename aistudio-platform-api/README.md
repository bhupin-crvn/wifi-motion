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
