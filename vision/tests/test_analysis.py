import unittest,json,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parents[1]))
import cv2,numpy as np
from analysis import analyze
class AnalysisTest(unittest.TestCase):
 def test_surface_and_units(self):
  a=np.full((256,256,3),145,np.uint8);cv2.circle(a,(50,50),5,(50,50,50),-1);cv2.line(a,(100,100),(140,140),(40,40,40),3);cv2.circle(a,(190,70),14,(50,50,50),-1)
  r,_,_=analyze(a,'surface');self.assertEqual(set(r['candidates']),{'particle','spot','scratch'});self.assertTrue(all('sizeUm' not in d for d in r['defects']))
  scaled,_,_=analyze(a,'surface',.5);self.assertEqual(scaled['defects'][0]['sizeUm'],r['defects'][0]['sizePx']*.5)
 def test_empty_unknown_invalid(self):
  r,_,_=analyze(np.full((256,256,3),145,np.uint8),'surface');self.assertEqual(r['quality'],'pass');self.assertEqual(r['defects'],[])
  for scale in [0,-1,float('nan')]:
   with self.assertRaises(ValueError):analyze(np.zeros((32,32,3),np.uint8),'surface',scale)
  with self.assertRaises(ValueError):analyze(np.zeros((2048,32,3),np.uint8),'map')
 def test_map_classes(self):
  for label in ['center','donut','edge-ring','scratch','near-full','none']:
   a=np.zeros((100,100,3),np.uint8);cv2.circle(a,(50,50),45,(127,127,127),-1)
   if label=='center':cv2.circle(a,(50,50),12,(255,255,255),-1)
   if label=='donut':cv2.circle(a,(50,50),25,(255,255,255),5)
   if label=='edge-ring':cv2.circle(a,(50,50),43,(255,255,255),3)
   if label=='scratch':cv2.line(a,(30,30),(70,70),(255,255,255),2)
   if label=='near-full':cv2.circle(a,(50,50),44,(255,255,255),-1)
   r,_,_=analyze(a,'map');self.assertIn(label,r['candidates'],r)
 def test_mixed(self):
  a=np.zeros((100,100,3),np.uint8);cv2.circle(a,(50,50),45,(127,127,127),-1);cv2.circle(a,(50,50),17,(255,255,255),-1);cv2.circle(a,(50,50),43,(255,255,255),6)
  r,_,_=analyze(a,"map");self.assertTrue(r["mixed"]);self.assertIn("center",r["candidates"]);self.assertIn("edge-ring",r["candidates"])
 def test_reproducible_pixel_only(self):
  a=np.full((256,256,3),145,np.uint8);cv2.circle(a,(55,55),7,(50,50,50),-1)
  r,overlay,_=analyze(a,'surface');r2,o2,_=analyze(a.copy(),'surface');self.assertEqual(r,r2);self.assertTrue(np.array_equal(overlay,o2))
if __name__=='__main__':unittest.main()
