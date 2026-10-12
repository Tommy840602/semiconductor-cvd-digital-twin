import sys,tempfile,time,unittest
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parents[1]))
from engine import Engine
class EngineTest(unittest.TestCase):
 def test_warm_repeat_and_invalid(self):
  import cv2,numpy as np
  raw=cv2.imencode('.png',np.full((128,128,3),145,np.uint8))[1].tobytes()
  with tempfile.TemporaryDirectory() as d:
   e=Engine(d)
   try:
    a=e.analyze(raw,'surface',None);b=e.analyze(raw,'surface',None);self.assertEqual(a,b);self.assertEqual(len(list(Path(d).glob('*.png'))),1)
    with self.assertRaises(ValueError):e.analyze(b'broken','surface',None)
    self.assertEqual(e.analyze(raw,'surface',None)['quality'],'pass')
   finally:e.close()
   self.assertFalse(e.process.is_alive())
