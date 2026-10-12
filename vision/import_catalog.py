"""Bounded JPG/PNG importer. No pickle or arbitrary archive extraction at runtime."""
import argparse, hashlib, json, zipfile
from pathlib import Path
import cv2
import numpy as np

def digest(b): return hashlib.sha256(b).hexdigest()
def png(image): return cv2.imencode('.png',image)[1].tobytes()
def safe_decode(data):
    if len(data)>2_000_000: raise ValueError('image-too-large')
    # JPEG source import runs in a memory-limited, offline container.
    a=cv2.imdecode(np.frombuffer(data,np.uint8),cv2.IMREAD_COLOR)
    if a is None or max(a.shape[:2])>1024: raise ValueError('invalid-dimensions')
    return a

def build(archive, output):
    if Path(archive).stat().st_size>5_000_000: raise ValueError("archive-too-large")
    output=Path(output); output.mkdir(parents=True,exist_ok=True)
    manifest=[]; labels=[]; counts={}
    with zipfile.ZipFile(archive) as z:
        if len(z.infolist())>2000 or sum(f.file_size for f in z.infolist())>20_000_000: raise ValueError('archive-too-large')
        if any(f.filename.lower().endswith(('.pkl','.pickle')) for f in z.infolist()): raise ValueError('pickle-not-supported')
        for f in sorted(z.infolist(),key=lambda x:x.filename):
            if not f.filename.lower().endswith(('.jpg','.png')): continue
            label=f.filename.split('/')[-2].lower().replace(' ','-')
            if label not in {'center','donut','edge-local','edge-ring','local','scratch','near-full','none','random'}: continue
            if counts.get(label,0)>=20: continue
            if f.file_size>2_000_000 or f.compress_size>2_000_000: raise ValueError('entry-too-large')
            raw=z.read(f); a=safe_decode(raw); data=png(a); h=digest(data)
            (output/(h+'.png')).write_bytes(data)
            sample=f.filename
            manifest.append(dict(kind='map',hash=h,width=a.shape[1],height=a.shape[0],mode='dataset-replay',datasetId='muhammedjunayed/wm811k-silicon-wafer-map-dataset-image',datasetVersion='1',sampleId=sample,sourceUrl='https://www.kaggle.com/datasets/muhammedjunayed/wm811k-silicon-wafer-map-dataset-image',license='CC-BY-SA-4.0',attribution='Muhammed Junayed; WM-811K / MIR Lab',originalHash=digest(raw),coordinates='image-x-right-y-down-px',calibration=None))
            labels.append(dict(hash=h,label=label,group=Path(sample).stem,mode='dataset-replay'))
            counts[label]=counts.get(label,0)+1
    if not manifest: raise ValueError("no-supported-map-images")
    # Only evaluation has ground-truth labels; the runtime manifest has no class labels.
    for n in range(40):
        rng=np.random.default_rng(n); a=np.clip(rng.normal(145,2,(256,256)),0,255).astype(np.uint8)
        truth=[];mask=np.zeros((256,256),np.uint8)
        if n%4!=0:
            for j,kind in enumerate(['particle','scratch','spot']):
                x=35+j*75; y=60+int(rng.integers(0,100))
                if kind=='scratch': cv2.line(a,(x-20,y-20),(x+20,y+20),65,3);cv2.line(mask,(x-20,y-20),(x+20,y+20),255,3)
                else:
                    radius=4 if kind=='particle' else 14
                    cv2.circle(a,(x,y),radius,55,-1);cv2.circle(mask,(x,y),radius,255,-1)
                truth.append(kind)
        a=cv2.cvtColor(a,cv2.COLOR_GRAY2BGR); data=png(a);h=digest(data);(output/(h+'.png')).write_bytes(data)
        manifest.append(dict(kind='surface',hash=h,width=256,height=256,mode='synthetic',datasetId='synthetic-surface-v1',datasetVersion='1',sampleId=str(n),seed=n,sourceUrl='',license='project-generated',attribution='CVD simulator',coordinates='image-x-right-y-down-px',calibration=None))
        labels.append(dict(hash=h,label=truth or ['none'],group='synthetic-'+str(n),mode='synthetic',mask=mask.tolist()))
    (output/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2))
    # Kept outside runtime catalog; never mounted into inference.
    (output.parent/'evaluation-labels.json').write_text(json.dumps(labels))
    return counts
if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('archive');p.add_argument('output'); a=p.parse_args();print(build(a.archive,a.output))
