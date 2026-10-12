import json,hashlib
from pathlib import Path
import cv2,numpy as np
from analysis import analyze

def evaluate(root,labels):
 inputs={x['hash']:x for x in json.loads((root/'manifest.json').read_text())};reports={};aliases={'edge-local':'edge-loc','local':'loc'}
 for row in labels:
  n=inputs[row['hash']]; split='validation' if int(hashlib.sha256(row['group'].encode()).hexdigest(),16)%5==0 else 'test'
  report=reports.setdefault(row['mode'],{'samples':0,'splits':{},'classes':{},'confusion':{},'iou':[]})
  a=cv2.imread(str(root/(row['hash']+'.png')));r,_,mask=analyze(a,n['kind'],n['calibration']);report['samples']+=1;report['splits'][split]=report['splits'].get(split,0)+1
  actual=row['label'] if isinstance(row['label'],list) else [aliases.get(row['label'],row['label'])];pred=r['candidates']
  for c in set(actual+pred):
   v=report['classes'].setdefault(c,{'tp':0,'fp':0,'fn':0});v['tp']+=int(c in actual and c in pred);v['fp']+=int(c not in actual and c in pred);v['fn']+=int(c in actual and c not in pred)
  k='+'.join(sorted(actual))+' -> '+'+'.join(sorted(pred));report['confusion'][k]=report['confusion'].get(k,0)+1
  if 'mask' in row:
   truth=np.array(row['mask'])>0;prediction=mask>0;union=(truth|prediction).sum();report['iou'].append(float((truth&prediction).sum()/union) if union else 1.)
 for report in reports.values():
  for c,v in report['classes'].items():
   p=v['tp']/max(1,v['tp']+v['fp']);r=v['tp']/max(1,v['tp']+v['fn']);v.update(precision=p,recall=r,f1=2*p*r/(p+r) if p+r else 0)
  report['meanMaskIoU']=sum(report['iou'])/len(report['iou']) if report['iou'] else None;del report['iou']
 return {'algorithmVersion':'opencv-rules-v1','trained':False,'splitPolicy':'sha256(source group) % 5; no derived sample crosses group','note':'Descriptive evaluation on a small sampled subset; no production accuracy claim.', 'reports':reports}
if __name__=='__main__':
 import sys
 print(json.dumps(evaluate(Path(sys.argv[1]),json.loads(Path(sys.argv[2]).read_text())),indent=2))
