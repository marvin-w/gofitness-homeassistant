#!/usr/bin/with-contenv bashio
# ==============================================================================
# GoFitness add-on entrypoint.
#
# The Go binary reads /data/options.json and /data directly, so there is nothing
# to translate here — bashio is used only for friendly startup logging. The
# Supervisor injects SUPERVISOR_TOKEN into the environment, which the app uses
# to publish sensors back to Home Assistant.
# ==============================================================================
set -e

bashio::log.info "Starting GoFitness..."

if bashio::config.has_value 'anthropic_api_key'; then
    bashio::log.info "AI calorie estimation: enabled"
else
    bashio::log.info "AI calorie estimation: disabled (local food table only)"
fi
bashio::log.info "Language: $(bashio::config 'default_language')"
bashio::log.info "Publish sensors: $(bashio::config 'publish_sensors')"

exec /usr/bin/gofitness
