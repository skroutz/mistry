#!/bin/bash
set -e

if [ -f cache/out.txt ]; then
  date +%S%N > artifacts/out.txt
else
  date +%S%N | tee cache/out.txt > artifacts/out.txt
fi
