# Tanjun

::: warning
This tool is still in development and is not yet ready for use.

Tanjun is a Dockerized Deployment Tool to deploy applications to external servers and cloud providers. 
It is designed to be straightforward to use and keep everything simple for the user.

It's similar to [Kamal Deploy](https://kamal-deploy.org/) but with a different approach.

## Requirements

A external reachable server with Docker installed.

## Commands

- `tanjun setup` - Setup Proxy Server on the remote server (one time).
- `tanjun deploy` - Deploy the current application to the remote server.
- `tanjun shell` - Open a shell to the remote server contain your application.
- `tanjun logs` - Show the logs of the application running on the remote server.
- `tanjun forward` - Forward the port of the application running on the remote server to your local machine.

## Example configuration

```yaml
# .tanjun.yml
# yaml-language-server: $schema=schema.json
server:
  # IP to oru server
  address: 127.0.0.1
# The name of the application, one server can contain multiple applications
name: app-name
# The image name to use to push and pull the image
imageName: "ghcr.io/shyim/test"
proxy:
  # The external reachable domain
  host: localhost
  # A healthcheck url to check if the application is running
  healthCheckUrl: /admin
app:
  env:
    # set a static environment value
    APP_URL:
      value: 'http://localhost'
  initialSecrets:
    # Generate a random string for the APP_SECRET environment variable and store it to keep it the same
    APP_SECRET:
      expr: 'randomString(32)'
  # Mount directories to the container
  mounts:
    - name: jwt
      path: config/jwt
  # Specify workers to run in background
  workers:
    worker:
      command: 'php bin/console messenger:consume async --time-limit=3600'
  # Specify cronjobs to run
  cronjobs:
    - schedule: '@every 5m'
      command: 'php bin/console scheduled-task:run --no-wait'
  # Hooks to run before the new container gets traffic
  hooks:
    # Initial setup
    setup: |
      ./vendor/bin/shopware-deployment-helper run
    # Executed after initial setup has been done once
    changed: |
      ./vendor/bin/shopware-deployment-helper run

services:
  # create an mysql database and sets a DATABASE_URL environment variable (based on key name)
  database:
    type: mysql:8.0
    settings:
      sql_mode: 'error_for_division_by_zero'
  # create a redis cache and sets a CACHE_URL environment variable (based on key name)
  cache:
    type: valkey:7.2
```