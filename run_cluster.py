#!ve/bin/python
import os
import uuid
import subprocess
from multiprocessing import Process

NUM_NODES=10

def f(s):
    p = subprocess.Popen("./reticulum -config=test/config%d.json" % (s), shell=True)
    sts = os.waitpid(p.pid, 0)[1]

for n in range(NUM_NODES):
    p = Process(target=f, args=(n,)).start()
