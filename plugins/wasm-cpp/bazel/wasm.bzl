load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")
load("@bazel_skylib//rules:copy_file.bzl", "copy_file")
load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_image",
    "container_push",
)

def wasm_libraries():
    http_archive(
        name = "com_google_absl",
        sha256 = "3a0bb3d2e6f53352526a8d1a7e7b5749c68cd07f2401766a404fb00d2853fa49",
        strip_prefix = "abseil-cpp-4bbdb026899fea9f882a95cbd7d6a4adaf49b2dd",
        url = "https://github.com/abseil/abseil-cpp/archive/4bbdb026899fea9f882a95cbd7d6a4adaf49b2dd.tar.gz",
        patch_args = ["-p1"],
        patches = ["//bazel:absl.patch"],
    )

    http_archive(
        name = "com_github_thalhammer_jwt_cpp",
        type = "tar.gz",
        urls = ["https://codeload.github.com/Thalhammer/jwt-cpp/tar.gz/refs/tags/v0.7.1"],
        sha256 = "e52f247d5e62fac5da6191170998271a70ce27f747f2ce8fde9b09f96a5375a4",
        strip_prefix = "jwt-cpp-0.7.1",
        build_file = "//bazel:jwt_cpp.BUILD",
    )

    http_archive(
        name = "com_github_mariusbancila_stduuid",
        type = "tar.gz",
        urls = ["https://codeload.github.com/mariusbancila/stduuid/tar.gz/refs/tags/v1.2.3"],
        sha256 = "b1176597e789531c38481acbbed2a6894ad419aab0979c10410d59eb0ebf40d3",
        strip_prefix = "stduuid-1.2.3",
        build_file = "//bazel:stduuid.BUILD",
    )

    http_file(
        name = "com_github_nlohmann_json_single_header",
        sha256 = "3b5d2b8f8282b80557091514d8ab97e27f9574336c804ee666fda673a9b59926",
        urls = [
            "https://github.com/nlohmann/json/releases/download/v3.7.3/json.hpp",
        ],
    )


    # import google test and cpp host for unit testing
    http_archive(
        name = "com_google_googletest",
        sha256 = "9dc9157a9a1551ec7a7e43daea9a694a0bb5fb8bec81235d8a1e6ef64c716dcb",
        strip_prefix = "googletest-release-1.10.0",
        urls = ["https://github.com/google/googletest/archive/release-1.10.0.tar.gz"],
    )

    PROXY_WASM_CPP_HOST_SHA = "ecf42a27fcf78f42e64037d4eff1a0ca5a61e403"
    PROXY_WASM_CPP_HOST_SHA256 = "9748156731e9521837686923321bf12725c32c9fa8355218209831cc3ee87080"

    http_archive(
        name = "proxy_wasm_cpp_host",
        sha256 = PROXY_WASM_CPP_HOST_SHA256,
        strip_prefix = "proxy-wasm-cpp-host-" + PROXY_WASM_CPP_HOST_SHA,
        url = "https://github.com/higress-group/proxy-wasm-cpp-host/archive/" + PROXY_WASM_CPP_HOST_SHA +".tar.gz",
    )

    http_archive(
        name = "boringssl",
        urls = ["https://github.com/google/boringssl/archive/648cbaf033401b7fe7acdce02f275b06a88aab5c.tar.gz"],
        strip_prefix = "boringssl-648cbaf033401b7fe7acdce02f275b06a88aab5c",
        patch_args = ["-p1"],
        patches = ["//bazel:boringssl.patch"],
    )

    native.bind(
        name = "ssl",
        actual = "@boringssl//:ssl",
    )

    native.bind(
        name = "protobuf",
        actual = "@com_google_protobuf//:protobuf",
    )

    http_archive(
        name = "com_googlesource_code_re2",
        urls = ["https://github.com/google/re2/archive/2020-07-06.tar.gz"],
        strip_prefix = "re2-2020-07-06",
        patch_args = ["-p1"],
        patches = ["//bazel:re2.patch"],
    )

    native.bind(
        name = "abseil_flat_hash_set",
        actual = "@com_google_absl//absl/container:flat_hash_set",
    )

    native.bind(
        name = "abseil_strings",
        actual = "@com_google_absl//absl/strings:strings",
    )

    native.bind(
        name = "abseil_time",
        actual = "@com_google_absl//absl/time:time",
    )

    native.bind(
        name = "protobuf",
        actual = "@com_google_protobuf//:protobuf",
    )

    http_archive(
        name = "com_github_google_jwt_verify",
        sha256 = "8964c2b3a833dc5fc2600b2768ea1e73a0fcf8a1ed9d2cbc5fa3387c4cdd5caa",
        urls = ["https://github.com/google/jwt_verify_lib/archive/26c22c0ce1bc607eec8fa5dd26b707378adc7a88.tar.gz"],
        strip_prefix = "jwt_verify_lib-26c22c0ce1bc607eec8fa5dd26b707378adc7a88"
    )
    
    
def declare_wasm_image_targets(name, wasm_file):
    # Rename to the spec compatible name.
    copy_file("copy_original_file", wasm_file, "plugin.wasm")
    container_image(
        name = "wasm_image",
        files = [":plugin.wasm"],
    )
    container_push(
        name = "push_wasm_image",
        format = "OCI",
        image = ":wasm_image",
        registry = "ghcr.io",
        repository = "istio-ecosystem/wasm-extensions/"+name,
        tag = "$(WASM_IMAGE_TAG)",
    )
