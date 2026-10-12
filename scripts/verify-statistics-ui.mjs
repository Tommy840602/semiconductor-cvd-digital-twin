import {spawn} from 'node:child_process';
import fs from 'node:fs/promises';
const child=spawn('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',['--headless','--no-first-run','--no-default-browser-check','--remote-debugging-port=9241','--user-data-dir=/tmp/cvd-disposition-ui-'+Date.now(),'about:blank'],{stdio:'ignore'});
const sleep=ms=>new Promise(r=>setTimeout(r,ms));let ws;
try {
 let tabs;for(let n=0;n<60;n++){try{tabs=await(await fetch('http://127.0.0.1:9241/json')).json();if(tabs.length)break;}catch{}await sleep(300);}
 ws=new WebSocket(tabs.find(t=>t.type==='page').webSocketDebuggerUrl);await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j});let seq=0;const pending=new Map(),errors=[];ws.onmessage=e=>{const m=JSON.parse(e.data);if(m.method==='Runtime.exceptionThrown')errors.push(m.params.exceptionDetails);if(pending.has(m.id)){const [r,j]=pending.get(m.id);pending.delete(m.id);m.error?j(Error(JSON.stringify(m.error))):r(m.result)}};
 const call=(method,params={})=>new Promise((r,j)=>{const id=++seq;pending.set(id,[r,j]);ws.send(JSON.stringify({id,method,params}));});
 const evaluate=async expression=>{const r=await call('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.exceptionDetails)throw Error(r.exceptionDetails.exception?.description);return r.result.value;};
 const wait=async expression=>{for(let n=0;n<200;n++){if(await evaluate(expression))return;await sleep(300);}throw Error(JSON.stringify(errors)+' Timeout '+expression+' '+await evaluate('document.body.innerText'));};
 const click=text=>evaluate(`(()=>{const b=[...document.querySelectorAll('button')].find(b=>b.textContent.trim()===${JSON.stringify(text)});if(!b)throw Error('missing button');b.click()})()`);
 await call('Runtime.enable');await call('Page.enable');await call('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false});await call('Page.navigate',{url:process.env.CVD_UI_URL||'http://127.0.0.1:22000/'});
 await wait('!!document.querySelector("input[autocomplete=username]")&&!document.querySelector(".auth-card button").disabled');
 await evaluate(`(()=>{const f=document.querySelectorAll('.auth-card input');[${JSON.stringify(process.env.CVD_UI_USER||'disposition.ee')},${JSON.stringify(process.env.CVD_UI_PASSWORD_FILE?(await fs.readFile(process.env.CVD_UI_PASSWORD_FILE,'utf8')).trim():'Disposition-Test-2026!')}].forEach((v,i)=>{f[i].value=v;f[i].dispatchEvent(new Event('input',{bubbles:true}))});document.querySelector('.auth-card').requestSubmit()})()`);
 await wait('document.querySelectorAll(".util-tools button").length===10');
 await wait('document.querySelectorAll(".stats-row").length===10');
 const charts=await evaluate(`({rows:document.querySelectorAll('.stats-row').length,outputs:document.querySelectorAll('.stats-output-row').length,donut:!!document.querySelector('.stats-donut'),text:document.querySelector('.stats-panel').innerText})`);
 if(charts.outputs!==10||!charts.donut||charts.text.includes('Cp'))throw Error('missing statistics');
 await call('Page.bringToFront');
 await evaluate(`(()=>{const s=document.querySelector('.stats-filter select');s.value='TEL-CVDTIN-08A-03';s.dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('.stats-row rect[role=button]').focus()})()`);
 await wait('!!document.querySelector(".stats-detail")');
 await call('Input.dispatchKeyEvent',{type:'keyDown',key:'Enter',code:'Enter',windowsVirtualKeyCode:13});
 const keyboardDetail=await evaluate('document.querySelector(".stats-detail").innerText');
 await sleep(31000);
 if(await evaluate('document.querySelector(".stats-filter select").value')!=='TEL-CVDTIN-08A-03')throw Error('refresh lost selection');
 const utilCheck=await evaluate('({cards:document.querySelectorAll(".util-tools button").length,text:document.querySelector(".util-panel").innerText})');
 for(const period of ['24h','7d','30d','today']){
  await evaluate(`(()=>{const s=document.querySelector('.util-heading select');s.value=${JSON.stringify(period)};s.dispatchEvent(new Event('change',{bubbles:true}))})()`);
  await wait('document.querySelectorAll(".util-tools button").length===10&&!document.querySelector(".util-panel").getAttribute("aria-busy").includes("true")');
 }
 // Force a failed query, then recover; rapid changes must retain the latest period.
 await evaluate(`(()=>{const original=window.fetch;let fail=true;window.fetch=(url,options)=>{if(String(url).includes('/equipment/statistics?period=7d')&&fail){fail=false;return Promise.reject(new Error('verification outage'))}return original(url,options)};const s=document.querySelector('.util-heading select');s.value='7d';s.dispatchEvent(new Event('change',{bubbles:true}))})()`);
 await wait('!!document.querySelector(".util-panel [role=alert]")');
 await evaluate('document.querySelector(".util-panel [role=alert] button").click()');
 await wait('document.querySelectorAll(".util-tools button").length===10');
 await evaluate(`(()=>{const s=document.querySelector('.util-heading select');for(const p of ['30d','24h','today']){s.value=p;s.dispatchEvent(new Event('change',{bubbles:true}))}})()`);
 await wait('document.querySelectorAll(".util-tools button").length===10&&document.querySelector(".util-heading select").value==="today"');
 await evaluate('document.querySelector(".stats-panel").scrollIntoView()');
 await evaluate(`(()=>{document.querySelector('.stats-row rect[role=button]').dispatchEvent(new MouseEvent('click',{bubbles:true}));document.querySelector('.stats-panel').scrollIntoView()})()`);
 await wait('!!document.querySelector(".stats-detail")');
 let shot=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/statistics-desktop.png',Buffer.from(shot.data,'base64'));
 await call('Emulation.setDeviceMetricsOverride',{width:390,height:844,deviceScaleFactor:1,mobile:true});await sleep(500);
 await evaluate('document.querySelector(".stats-panel").scrollIntoView()');
 await call('Emulation.setTouchEmulationEnabled',{enabled:true});
 const point=await evaluate(`(()=>{const r=document.querySelectorAll('.stats-row')[1].querySelector('rect[role=button]').getBoundingClientRect();return {x:Math.min(r.left+6,innerWidth-20),y:r.top+r.height/2}})()`);
 await call('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[point]});await call('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});
 await wait('document.querySelector(".stats-detail")?.textContent.includes("TEL-CVDTIN-08A-02")');
 const mobile=await evaluate('({viewport:innerWidth,width:document.documentElement.scrollWidth})');if(mobile.width>mobile.viewport+1)throw Error('mobile overflow '+JSON.stringify(mobile));
 shot=await call('Page.captureScreenshot',{format:'png'});await fs.writeFile('.runtime/statistics-mobile.png',Buffer.from(shot.data,'base64'));
 await call('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false});
 await wait('!!document.querySelector(".cr-enter")');
 const expected=await evaluate('document.querySelector(".cr-enter b").textContent');
 await evaluate('document.activeElement?.blur()');
 await call('Input.dispatchKeyEvent',{type:'keyDown',key:'Enter',code:'Enter',windowsVirtualKeyCode:13});
 await call('Input.dispatchKeyEvent',{type:'keyUp',key:'Enter',code:'Enter',windowsVirtualKeyCode:13});
 await wait('!document.querySelector(".cr-enter")&&!document.querySelector(".cleanroom")');
 const selected=await evaluate('document.querySelector(".nav-equipment b").textContent');
 const processVisible=await evaluate('document.body.innerText.includes("設備配置")');
 if(selected!==expected||!processVisible)throw Error(JSON.stringify({expected,selected,processVisible}));
 if(errors.length)throw Error(JSON.stringify(errors));
 await call('Input.dispatchKeyEvent',{type:'keyDown',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});
 await call('Input.dispatchKeyEvent',{type:'keyUp',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});
 await wait('!!document.querySelector(".cleanroom canvas")');
 const returnedToFleet=await evaluate('!!document.querySelector(".cleanroom canvas")');
 const proof={expected,selected,processVisible,returnedToFleet,utilCheck,charts,keyboardDetail,mobile,runtimeErrors:errors.length};
 await fs.writeFile('.runtime/statistics-ui-proof.json',JSON.stringify(proof,null,2));console.log(JSON.stringify(proof));
}finally{ws?.close();child.kill('SIGTERM');}
