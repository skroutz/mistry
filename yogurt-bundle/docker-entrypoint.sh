#!/bin/bash
set -e

echo "$GEMFILE_CONTENTS" > Gemfile
echo "$LOCKFILE_CONTENTS" > Gemfile.lock

bundle install --deployment --without=test development --path .
