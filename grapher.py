import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import statistics
import os
import re
#os.chdir("/home/bruno/Desktop/tests/0logger3servertest")
print(os.getcwd())
dir_files=(os.listdir())
pattern=r"\d{1}c-latency.out"
logfiles=[]
for file in dir_files:
    if re.search(pattern,file):
        logfiles.append(int(re.sub("[ -/,:-~]+","", file)))
logfiles.sort()
print(logfiles)

average_latency=[None for i in range(0,max(logfiles)+1)]

for numb_clients in logfiles:
    f = open(f"{os.getcwd()}/{numb_clients}c-latency.out", "r")
    latency=[]
    for line in f:
        latency.append(int(line))

    average_latency[numb_clients]=statistics.mean(latency)/1000000
    print(average_latency)
s= pd.Series(average_latency)
s.dropna().plot()
plt.show()

