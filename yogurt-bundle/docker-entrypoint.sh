#!/bin/bash
set -e

echo "$GEMFILE" > cache/Gemfile
echo "$LOCKFILE" > cache/Gemfile.lock

bundle install --gemfile=tmp/Gemfile --deployment --path /data/artifacts
bundle clean
