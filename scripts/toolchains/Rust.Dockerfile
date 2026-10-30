ARG RUST_IMAGE=rust:1.95.0-bookworm@sha256:4c2fd73ef19c5ef9d54bee03b06b2839a392604fbfcd578ed948b71b37c1d7fb
FROM ${RUST_IMAGE}
ADD --checksum=sha256:b761e3a0721dbae9c09a0059e5fdb2bf917d1b4a8a7b430fb3b5aafb0984b2c4 https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-34/wasi-sdk-34.0-x86_64-linux.tar.gz /tmp/wasi-sdk.tar.gz
RUN mkdir /opt/wasi-sdk && tar -xzf /tmp/wasi-sdk.tar.gz --strip-components=1 -C /opt/wasi-sdk && rm /tmp/wasi-sdk.tar.gz
ENV CC_wasm32_wasip1=/opt/wasi-sdk/bin/clang
ENV AR_wasm32_wasip1=/opt/wasi-sdk/bin/llvm-ar
ENV WASI_SYSROOT=/opt/wasi-sdk/share/wasi-sysroot
RUN rustup target add wasm32-wasip1
