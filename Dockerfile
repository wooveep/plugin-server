# Runtime/toolchain and Python wheels are both immutable, independently locked inputs.
ARG PYTHON_IMAGE=python:3.13.5-slim-bookworm@sha256:4c2cf9917bd1cbacc5e9b07320025bdb7cdf2df7b0ceaccb55e9dd7e30987419
FROM ${PYTHON_IMAGE} AS builder
WORKDIR /work
COPY requirements.lock ./
RUN python3 -m pip install --no-cache-dir --require-hashes -r requirements.lock
COPY artifact_contract.py artifact_import.py artifact_packaging.py artifact_retention.py artifact_storage.py mf_canonical.py plugin_schema_conversion.py plugin_server.py upload_plugin.py pull_plugins.py generate_metadata.py ./
COPY artifact_schemas/ ./artifact_schemas/
COPY scripts/ ./scripts/
ARG LEGACY_PLUGINS_DIR=plugins
COPY ${LEGACY_PLUGINS_DIR}/ ./plugins/
ARG PACKAGES_DIR=packages
COPY ${PACKAGES_DIR}/ ./packages/
RUN python3 scripts/seed-artifact-packages.py --source packages --destination plugins --policy packages/policy.json
RUN python3 scripts/export-artifact-inventory.py --plugins-dir plugins --output artifact-inventory.json

FROM ${PYTHON_IMAGE}
WORKDIR /opt/plugin-server
COPY --from=builder /usr/local/lib/python3.13/site-packages/ /usr/local/lib/python3.13/site-packages/
COPY --from=builder /work/*.py ./
COPY --from=builder /work/artifact_schemas/ ./artifact_schemas/
COPY --from=builder /work/requirements.lock ./requirements.lock
COPY --from=builder /work/packages/policy.json ./policy.json
COPY --from=builder /work/artifact-inventory.json ./artifact-inventory.json
COPY --from=builder /work/plugins/ /usr/share/plugin-server/plugins/
COPY --from=builder /work/.plugins-artifact-state/ /usr/share/plugin-server/.plugins-artifact-state/
ENV PLUGIN_SERVER_PLUGINS_DIR=/usr/share/plugin-server/plugins
ENV PLUGIN_SERVER_STATE_DIR=/usr/share/plugin-server/.plugins-artifact-state
ENV PLUGIN_SERVER_POLICY_FILE=/opt/plugin-server/policy.json
VOLUME ["/usr/share/plugin-server"]
EXPOSE 8080
CMD ["python3", "/opt/plugin-server/plugin_server.py"]
