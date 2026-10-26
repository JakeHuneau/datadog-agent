"""Lays out Windows builds of tests for e2e suites to run them remotely, as `go test -c` would have built them.

Linux x86_64 is the only non-Windows host able to build their Windows resources, see
//bazel/rules/windows:linux_x86_64_toolchain. with_cfg applies target_compatible_with to the target it configures for
Windows too, so callers tag suites manual and constrain their pkg_install instead.
"""

load("@bazel_skylib//lib:paths.bzl", "paths")
load("@rules_pkg//pkg:providers.bzl", "PackageFilesInfo")
load("@with_cfg.bzl", "with_cfg")

def _impl(ctx):
    dest_src_map = {}
    for test, dest in ctx.attr.tests.items():
        dest_src_map[dest] = test[DefaultInfo].files_to_run.executable
        testdata = test.label.package + "/testdata/"
        for f in test[DefaultInfo].default_runfiles.files.to_list():
            if f.short_path.startswith(testdata):
                dest_src_map[paths.join(paths.dirname(dest), f.short_path.removeprefix(test.label.package + "/"))] = f
    return [
        DefaultInfo(files = depset(dest_src_map.values())),
        PackageFilesInfo(attributes = {"mode": "0755"}, dest_src_map = dest_src_map),
    ]

_windows_testsuites = rule(
    implementation = _impl,
    doc = "Maps Windows builds of tests, and their `testdata` next to them, to destinations for `pkg_install`.",
    attrs = {
        "tests": attr.label_keyed_string_dict(
            doc = "Tests to build for Windows, mapped to their destination",
            mandatory = True,
        ),
    },
)

windows_testsuites, _ = with_cfg(_windows_testsuites, extra_providers = [PackageFilesInfo]).set("platforms", [Label("//bazel/platforms:windows_x86_64")]).build()
