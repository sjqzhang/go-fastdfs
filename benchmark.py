# -*- coding: utf-8 -*-
import requests

import gevent


from Queue import Queue

q=Queue()

import commands

from gevent import monkey

monkey.patch_all()
import os
out=commands.getoutput('find . -type f')

lines=out.split("\n")

for i in lines:
    q.put(i)


def task():
    while True:
        name=q.get(block=False)
        if name=="":
            break
        url = 'http://10.1.5.20:8080/group1/upload'
        files = {'file': open(name, 'rb')}
        options = {'output': 'json', 'path': '', 'scene': ''}
        try:
            r = requests.post(url, files=files)
        except Exception as er:
            print(er)

th=[]
for i in range(200):
    th.append(gevent.spawn(task))
gevent.joinall(th)

