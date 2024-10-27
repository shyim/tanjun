#syntax=docker/dockerfile:1.4

# pin versions
FROM ghcr.io/shopware/docker-base:8.3 as base-image
FROM ghcr.io/friendsofshopware/shopware-cli:latest-php-8.2 as shopware-cli

# build

FROM shopware-cli as build

COPY --link . /src
WORKDIR /src

RUN mkdir -p custom/plugins && mkdir -p custom/static-plugins
RUN shopware-cli project ci /src

# build final image

FROM base-image

COPY --from=build --chown=www-data /src /var/www/html