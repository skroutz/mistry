#!/bin/bash
#HACK: mistry will attempt to copy all artifacts out of the container
# Therefore we need to leave something behind. This might or might not be
# the intented behaviour. Revisit in the future.
touch /data/artifacts/foo
stat /koko/lala.txt
