#!/bin/sh
#
# Docker container Entrypoint script
#


[ -z "${CONFIG}" ] && CONFIG="subscriber.conf"

[ -z "${REDIS_ADDRESS}" ] && REDIS_ADDRESS="127.0.0.1"
[ -z "${REDIS_PORT}" ] && REDIS_PORT="6379"


./subscriber -verbose -config "${CONFIG}" -redis-address "${REDIS_ADDRESS}:${REDIS_PORT}"
