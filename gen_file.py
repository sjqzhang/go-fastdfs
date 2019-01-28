# -*- coding: utf-8 -*-
import os
j=0
for i in range(0,1000000):
    if i%1000==0:
        j=i
        os.system('mkdir %s'%(i))
    with open('%s/%s.txt'%(j,i),'w+') as f:
        f.write(str(i))
