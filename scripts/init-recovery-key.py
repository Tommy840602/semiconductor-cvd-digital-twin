#!/usr/bin/env python3
"""Create a shared device approval-signing key once; never print or overwrite it."""
import os,secrets,pathlib
path=pathlib.Path(os.getenv('CVD_RECOVERY_KEY_FILE','.runtime/secrets/recovery-key'))
path.parent.mkdir(parents=True,exist_ok=True)
try:
 fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
except FileExistsError:
 print('Recovery signing key already exists; preserved.')
else:
 with os.fdopen(fd,'w') as f:f.write(secrets.token_hex(32)+'\n')
 print('Recovery signing key created. Keep it with deployment secrets.')
