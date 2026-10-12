import unittest,tempfile,zipfile,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parents[1]))
from import_catalog import build
class ImportTest(unittest.TestCase):
 def test_pickle_rejected_without_loading(self):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d)/'input.zip'
   with zipfile.ZipFile(p,'w') as z:z.writestr('evil.pkl',b'not deserialized')
   with self.assertRaisesRegex(ValueError,'pickle-not-supported'):build(p,Path(d)/'catalog')
 def test_expansion_rejected(self):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d)/'input.zip'
   with zipfile.ZipFile(p,'w',zipfile.ZIP_DEFLATED) as z:z.writestr('large.jpg',b'0'*20_000_001)
   with self.assertRaisesRegex(ValueError,'archive-too-large'):build(p,Path(d)/'catalog')
