#!/bin/bash
cd book

git add *.md

gitbook build 

cp -f  _book/*.html ../


cd ..


git add *.html

git commit -m "qa"

git push

git push gitee
