#!/bin/bash
set -e

## these 2 will go away once we're on master
git fetch
git checkout origin/builder-assets

bundle install # only run if needed
yarn install
script/lnconfs.rb
script/mock_ymls.rb

if [ -d /data/cache/sprockets_cache ]; then
	mv /data/cache/sprockets_cache tmp
fi

RAILS_ENV=production bundle exec rake assets:precompile

mv tmp /data/cache/sprockets_cache
cp -r public/assets/* builds/artifacts/
