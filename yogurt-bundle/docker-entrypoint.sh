#!/bin/bash
set -e

echo "$GEMFILE_CONTENTS" > cache/Gemfile
echo "$LOCKFILE_CONTENTS" > cache/Gemfile.lock

bundle install --gemfile=tmp/Gemfile --deployment --path /data/artifacts
bundle clean
