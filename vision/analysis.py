"""Pixel-only inspection. Dataset labels are deliberately absent from this API."""
import math
import cv2
import numpy as np
cv2.setNumThreads(1)
VERSION='opencv-rules-v1'
PARAMS={'surfaceContrast':25,'minAreaPx':5,'spotAreaPx':150,'scratchAspect':4,'maxDefects':1000}

def analyze(image,kind,calibration=None):
    if image is None or image.dtype!=np.uint8 or image.ndim!=3 or image.shape[2]!=3 or not 8<=min(image.shape[:2]) or max(image.shape[:2])>1024:
        raise ValueError('invalid-image')
    if calibration is not None and (not isinstance(calibration,(float,int)) or not math.isfinite(calibration) or calibration<=0):raise ValueError('invalid-scale')
    if kind not in ('map','surface'):raise ValueError('invalid-kind')
    h,w=image.shape[:2]; overlay=image.copy(); gray=cv2.cvtColor(image,cv2.COLOR_BGR2GRAY)
    candidates=[]; features={}; defects=[]
    if kind=='map':
        # Public source uses rendered map colors. Also accept canonical 0/127/255 grids.
        hsv=cv2.cvtColor(image,cv2.COLOR_BGR2HSV)
        color=hsv[:,:,1]>70
        if color.sum()>10:
            # WM maps render failed dies yellow/red and good dies blue.
            wafer=color & (hsv[:,:,0]>=15) & (hsv[:,:,0]<=100)
            mask=(wafer & (hsv[:,:,0]<35) & (hsv[:,:,2]>150)).astype(np.uint8)*255
        else:
            wafer=gray>40; mask=((gray>190)&wafer).astype(np.uint8)*255
        yy,xx=np.nonzero(wafer)
        if not len(xx): raise ValueError('missing-wafer')
        cx=(xx.min()+xx.max())/2;cy=(yy.min()+yy.max())/2
        radius=max((xx.max()-xx.min())/2,(yy.max()-yy.min())/2,1)
        y,x=np.nonzero(mask); ratio=len(x)/len(xx)
        features={'failedFraction':round(ratio,5),'failedPixels':len(x)}
        if len(x)==0:candidates=['none']
        elif ratio>.8:candidates=['near-full']
        elif len(x)>=5:
            r=np.hypot(x-cx,y-cy)/radius
            center=float(np.mean(r<.4));edge=float(np.mean(r>.75))
            bins=np.unique(((np.arctan2(y-cy,x-cx)+np.pi)/(2*np.pi)*12).astype(int)).size
            cov=np.linalg.eigvalsh(np.cov(np.stack([x,y])))
            aspect=float(cov[-1]/max(cov[0],.01))
            features.update(centerFraction=round(center,4),edgeFraction=round(edge,4),angularBins=int(bins),elongation=round(aspect,3))
            all_y,all_x=np.nonzero(wafer);all_r=np.hypot(all_x-cx,all_y-cy)/radius
            center_density=float(np.sum(r<.4)/max(1,np.sum(all_r<.4)))
            edge_density=float(np.sum(r>.8)/max(1,np.sum(all_r>.8)))
            if center>.6 or center_density>.65:candidates.append('center')
            if edge>.65 or edge_density>.6:candidates.append('edge-ring' if bins>=10 else 'edge-loc')
            if .35<np.mean(r)<.8 and np.std(r)<.17 and bins>=10:candidates.append('donut')
            if aspect>9:candidates.append('scratch')
            if not candidates:
                if np.std(x)/radius<.3 and np.std(y)/radius<.3:candidates=['loc']
                elif bins>=10 and np.std(r)>.22:candidates=['random']
        if not candidates:candidates=['uncertain']
    else:
        # Robust background estimate preserves broad dark spots and thin scratches.
        background=cv2.medianBlur(gray,51)
        residual=cv2.absdiff(gray,background)
        mask=(residual>PARAMS['surfaceContrast']).astype(np.uint8)*255
        features={'contrastThreshold':PARAMS['surfaceContrast']}
    count,labels,stats,centroids=cv2.connectedComponentsWithStats(mask,8)
    for i in range(1,count):
        x,y,bw,bh,area=map(int,stats[i]);
        if area<(1 if kind=='map' else PARAMS['minAreaPx']):continue
        points=np.column_stack(np.nonzero(labels==i)[::-1]).astype(np.float32)
        rect=cv2.minAreaRect(points); a,b=sorted(rect[1]); aspect=b/max(a,1)
        typ='region' if kind=='map' else ('scratch' if aspect>PARAMS['scratchAspect'] else ('spot' if area>=PARAMS['spotAreaPx'] else 'particle'))
        d={'id':len(defects)+1,'type':typ,'x':x,'y':y,'width':bw,'height':bh,'areaPx':area,'sizePx':max(bw,bh)}
        if calibration is not None:d['sizeUm']=round(max(bw,bh)*calibration,4)
        defects.append(d);cv2.rectangle(overlay,(x,y),(x+bw-1,y+bh-1),(0,0,255),1)
    if len(defects)>PARAMS['maxDefects']:raise ValueError('too-many-defects')
    if kind=='surface':candidates=sorted({d['type'] for d in defects}) or ['none']
    quality='uncertain' if 'uncertain' in candidates else ('pass' if candidates==['none'] else 'review')
    return dict(algorithmVersion=VERSION,opencvVersion=cv2.__version__,parameters=PARAMS.copy(),kind=kind,width=w,height=h,coordinates='image-x-right-y-down-px',quality=quality,candidates=candidates,mixed=len(candidates)>1,features=features,defects=defects),overlay,mask
