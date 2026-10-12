"""Create an independent worker credential without replacing existing credentials."""
from pathlib import Path
import secrets,os
root=Path(__file__).resolve().parents[1]
p=root/'.runtime/secrets/vision-token';p.parent.mkdir(parents=True,exist_ok=True)
if not p.exists():
 fd=os.open(p,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
 with os.fdopen(fd,'w') as f:f.write(secrets.token_urlsafe(48))
if len(p.read_text().strip())<32:raise SystemExit('Existing vision token is too short; replace it deliberately.')
p.chmod(0o600)
print('Vision worker credential ready (value not displayed).')
