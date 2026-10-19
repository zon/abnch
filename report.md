Fail `abnch set config` without a URL and without one in the config

`abnch set config` now consistently exits non-zero, names `--url` in its
error, and leaves the config untouched when neither the flag nor the config
provides a URL. The existing command already behaved this way; the tests now
cover the case where a config exists but has no url, asserting its contents
stay unchanged.

Tests cover the missing-URL failure both with no config and with an existing
config that has no url.

repo-config-GMErg5m
