# Source compilation and upload ingestion finish before image publication.
# This stage copies the registered immutable store and checks catalog completeness.
ARG PYTHON_IMAGE=python:3.13.5-slim-bookworm@sha256:4c2cf9917bd1cbacc5e9b07320025bdb7cdf2df7b0ceaccb55e9dd7e30987419
FROM ${PYTHON_IMAGE} AS builder
WORKDIR /work
COPY requirements.lock ./
RUN python3 -m pip install --no-cache-dir --require-hashes -r requirements.lock
COPY *.py ./
COPY artifact_schemas/ ./artifact_schemas/
COPY scripts/seed-registered-store.py scripts/verify-release-store.py scripts/export-artifact-inventory.py ./scripts/
COPY plugins/release/catalog.json ./source-catalog.json
COPY dist/plugins/ ./plugins/
COPY dist/.plugins-artifact-state/ ./.plugins-artifact-state/
COPY dist/policy.json ./policy.json
RUN python3 scripts/verify-release-store.py --plugins-dir plugins --catalog source-catalog.json --output plugin-catalog.json
RUN python3 scripts/export-artifact-inventory.py --plugins-dir plugins --output artifact-inventory.json

FROM ${PYTHON_IMAGE}
WORKDIR /opt/plugin-server
COPY --from=builder /usr/local/lib/python3.13/site-packages/ /usr/local/lib/python3.13/site-packages/
COPY --from=builder /work/*.py ./
COPY --from=builder /work/artifact_schemas/ ./artifact_schemas/
COPY --from=builder /work/scripts/ ./scripts/
COPY --from=builder /work/requirements.lock /work/policy.json /work/artifact-inventory.json /work/plugin-catalog.json /work/source-catalog.json ./
COPY --from=builder /work/plugins/ /usr/share/plugin-server/plugins/
COPY --from=builder /work/.plugins-artifact-state/ /usr/share/plugin-server/.plugins-artifact-state/
ENV PLUGIN_SERVER_PLUGINS_DIR=/usr/share/plugin-server/plugins
ENV PLUGIN_SERVER_STATE_DIR=/usr/share/plugin-server/.plugins-artifact-state
ENV PLUGIN_SERVER_POLICY_FILE=/opt/plugin-server/policy.json
VOLUME ["/usr/share/plugin-server"]
EXPOSE 8080
CMD ["python3", "/opt/plugin-server/plugin_server.py"]
