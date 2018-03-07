#!/bin/bash
set -e

## these 2 will go away once we're on master
git fetch
git checkout builder-assets

git pull

bundle install # only run if needed
yarn install
script/lnconfs.rb
script/mock_ymls.rb
/bin/bash
#RAILS_ENV=production bundle exec rake assets:precompile
mv tmp/ /data/cache
