#!/bin/bash

git add submit.sh
cd book

git add *.md

gitbook build 

cp -f  _book/*.html _book/search_index.json ../


cd ..


git add *.html

git commit -m "qa"

git push

git push gitee
