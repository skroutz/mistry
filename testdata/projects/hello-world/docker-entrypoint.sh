#!/bin/bash
set -e

if [ -f cache/foo.txt ]; then
  date +%S%N > artifacts/foo.txt
else
  date +%S%N | tee cache/foo.txt > artifacts/foo.txt
fi
