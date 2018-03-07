#!/bin/bash
set -e

# TODO: these 2 will go away once we're on master
git fetch
git checkout origin/builder-assets

bundle install # only run if needed
yarn install
# TODO: commit those
script/lnconfs.rb
script/mock_ymls.rb

if [ -d /data/cache/sprockets_cache ]; then
	ln -sf /data/cache/sprockets_cache tmp
fi

RAILS_ENV=production bundle exec rake assets:precompile

# TODO: introduce build cache
NODE_ENV=production yarn build

ln -sf public/assets /data/artifacts
