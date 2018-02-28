#!/bin/bash
set -e

git fetch && git checkout builder-assets && git pull
bundle install
yarn install
script/lnconfs.rb
script/mock_ymls.rb
RAILS_ENV=production bundle exec rake assets:precompile
