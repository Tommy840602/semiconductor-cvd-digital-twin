import hashlib,json,os,sys,uuid
from pathlib import Path
import cv2
from analysis import analyze
image=cv2.imread(sys.argv[1]);result,overlay,_=analyze(image,sys.argv[2],json.loads(sys.argv[3]))
data=cv2.imencode('.png',overlay)[1].tobytes();h=hashlib.sha256(data).hexdigest();root=Path(sys.argv[4]);root.mkdir(parents=True,exist_ok=True)
tmp=root/(str(uuid.uuid4())+'.tmp')
with tmp.open('wb') as f:f.write(data);f.flush();os.fsync(f.fileno())
os.replace(tmp,root/(h+'.png'));result['overlayHash']=h
print(json.dumps(result,allow_nan=False))
