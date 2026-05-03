#!/bin/bash
set -e
cd frontend
npm install
npm run build
mkdir -p ../app/src/main/assets/www
cp -r dist/* ../app/src/main/assets/www/
echo "Frontend built and copied to Android assets"
