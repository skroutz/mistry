#!/bin/bash
set -e

>&2 echo "this is stderr"
echo "this is stdout"
missing_command

exit 0
