import {t} from '../i18n.js';
import {glowLevel} from '../statusGlow.js';
import * as T from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {OrbitControls} from 'three/addons/controls/OrbitControls.js';
import {mergeGeometries} from 'three/addons/utils/BufferGeometryUtils.js';
import {buildEnvironment} from './environment.js';
import {positions,route,move,status} from './navigation.js';

export async function createScene(host,state,{onSelect,onProgress,onError,onMetrics,signal}) {
 const renderer=new T.WebGLRenderer({antialias:true,alpha:false});
 renderer.setPixelRatio(Math.min(devicePixelRatio,1.5));renderer.setClearColor('#e9f1ef');
 renderer.outputColorSpace=T.SRGBColorSpace;renderer.toneMapping=T.ACESFilmicToneMapping;renderer.toneMappingExposure=1;
 host.appendChild(renderer.domElement);
 const scene=new T.Scene(), camera=new T.PerspectiveCamera(42,1,.1,200);
 const controls=new OrbitControls(camera,renderer.domElement);controls.enableDamping=true;controls.maxPolarAngle=Math.PI*.46;controls.minDistance=10;controls.maxDistance=115;
 const materials=new Set(),geometries=new Set(),textures=new Set();let dead=false,raf=0,last=0,active=false,metricsTime=0,frames=0,elapsed=0;
 const listeners=[];const listen=(target,event,fn)=>{target.addEventListener(event,fn);listeners.push(()=>target.removeEventListener(event,fn));};
 const motion=matchMedia('(prefers-reduced-motion: reduce)');let reduced=motion.matches;listen(motion,'change',e=>{reduced=e.matches;});
 const mat=(color)=>{const m=new T.MeshStandardMaterial({color,roughness:.75});materials.add(m);return m;};
 const add=(geo,m,x,y,z,parent=scene)=>{geometries.add(geo);const mesh=new T.Mesh(geo,m);mesh.position.set(x,y,z);parent.add(mesh);return mesh;};
 const box=(w,h,d,m,x,y,z,parent)=>add(new T.BoxGeometry(w,h,d),m,x,y,z,parent);
 const white=mat('#f7faf9'),teal=mat('#336e69'),floor=mat('#dce6e3'),dark=mat('#183f44'),blue=mat('#8fb8c3');
 renderer.shadowMap.enabled=true;renderer.shadowMap.type=T.PCFSoftShadowMap;
 scene.add(new T.HemisphereLight('#e7f5ff','#6a8080',1.5));
 const sun=new T.DirectionalLight('#f5fbff',2.1);sun.position.set(9,24,8);sun.castShadow=true;sun.shadow.mapSize.set(2048,2048);Object.assign(sun.shadow.camera,{left:-30,right:30,top:30,bottom:-30,near:1,far:65});sun.shadow.bias=-.0004;sun.shadow.normalBias=.025;scene.add(sun);
 const roof=buildEnvironment(scene,{materials,geometries,textures});
 function label(text,width=6){const c=document.createElement('canvas');c.width=768;c.height=96;const ctx=c.getContext('2d');ctx.fillStyle='#214c4b';ctx.font='600 36px system-ui';ctx.textAlign='center';ctx.fillText(text,384,62);const texture=new T.CanvasTexture(c);textures.add(texture);const material=new T.SpriteMaterial({map:texture,depthTest:false});materials.add(material);const sprite=new T.Sprite(material);sprite.scale.set(width,width/8,1);scene.add(sprite);return sprite;}
 let aisleLabel=label(t('aisle'),7);aisleLabel.position.set(0,.2,21);
 function setLocale(){if(dead)return;scene.remove(aisleLabel);const material=aisleLabel.material;material.map.dispose();textures.delete(material.map);material.dispose();materials.delete(material);aisleLabel=label(t('aisle'),7);aisleLabel.position.set(0,.2,21);}
 const avatar=new T.Group();scene.add(avatar);
 box(.55,.72,.36,white,0,1.05,0,avatar);
 add(new T.SphereGeometry(.27,16,12),white,0,1.65,0,avatar);
 box(.34,.13,.09,dark,0,1.68,.23,avatar);box(.34,.13,.10,blue,0,1.54,.22,avatar);
 const limbs=[];
 for(const side of [-1,1]){
  const leg=new T.Group();leg.position.set(side*.16,.73,0);avatar.add(leg);box(.23,.55,.25,white,0,-.28,0,leg);box(.26,.16,.38,teal,0,-.6,.06,leg);limbs.push(leg);
  const arm=new T.Group();arm.position.set(side*.37,1.36,0);avatar.add(arm);box(.18,.48,.20,white,0,-.23,0,arm);add(new T.SphereGeometry(.11,10,8),blue,0,-.51,0,arm);limbs.push(arm);
 }
 avatar.traverse(o=>{if(o.isMesh)o.castShadow=true;});
 let tools,markers=[],halos=[],lamps=[],fleet=[],input={x:0,z:0},walkPhase=0,currentView='';const ray=new T.Raycaster(),pointer=new T.Vector2();let down=null;
 function resize(){const w=host.clientWidth,h=host.clientHeight;if(!w||!h)return;renderer.setSize(w,h);camera.aspect=w/h;camera.updateProjectionMatrix();if(state.view==='overview')setCamera(true);}
 function setCamera(force=false){if(!force&&currentView===state.view)return;currentView=state.view;controls.enabled=state.view==='overview';roof.visible=!controls.enabled;if(controls.enabled){camera.position.set(31,36,35);controls.target.set(0,0,0);if(camera.aspect<.8)camera.position.set(0,82,38);controls.update();}}
 function stop(){active=false;cancelAnimationFrame(raf);last=0;input={x:0,z:0};}
 function dispose(){if(dead)return;dead=true;stop();observer.disconnect();listeners.forEach(f=>f());controls.dispose();geometries.forEach(g=>g.dispose());materials.forEach(m=>m.dispose());textures.forEach(t=>t.dispose());sun.shadow.map?.dispose();renderer.dispose();renderer.forceContextLoss();renderer.domElement.remove();}
 const observer=new ResizeObserver(resize);observer.observe(host);
 listen(renderer.domElement,'webglcontextlost',e=>{e.preventDefault();if(!dead){stop();onError(t('sceneInterrupted'));}});
 signal?.addEventListener('abort',dispose,{once:true});
 const api={dispose,setLocale,setActive(value){if(dead)return;if(value&&!active){active=true;last=0;raf=requestAnimationFrame(frame);}else if(!value)stop();},setInput(v){input=v;},setFleet(v){fleet=v;},metrics(){return {active,reducedMotion:reduced,glows:markers.map((m,i)=>({id:positions[i].id,color:m.material.color.getHexString(),intensity:m.material.emissiveIntensity,lampIntensity:lamps[i].material.emissiveIntensity,haloOpacity:halos[i].material.opacity})),...renderer.info.render,memory:{...renderer.info.memory},resources:{materials:materials.size,textures:textures.size,geometries:geometries.size},position:{x:state.x,z:state.z},mode:state.mode,view:state.view,waypoint:state.waypoint,statuses:positions.map(p=>({id:p.id,...status(fleet.find(f=>f.equipmentId===p.id))})),targets:positions.map(p=>{const v=new T.Vector3(p.x,1.2,p.z).project(camera);return {id:p.id,x:(v.x+1)/2*host.clientWidth,y:(1-v.y)/2*host.clientHeight};})};}};
 try {
  const gltf=await new GLTFLoader().loadAsync('/models/tel-cvd.glb',e=>onProgress(e.total?Math.round(e.loaded/e.total*100):0));
  if(dead){gltf.scene.traverse(o=>{o.geometry?.dispose();if(o.material)o.material.dispose();});return api;}
  gltf.scene.updateMatrixWorld(true);
  const parts=[];const conversion=new T.Matrix4().makeRotationX(-Math.PI/2);
  gltf.scene.traverse(o=>{if(!o.isMesh)return;const g=o.geometry.clone().applyMatrix4(o.matrixWorld).applyMatrix4(conversion);const flat=g.index?g.toNonIndexed():g;flat.computeVertexNormals();for(const key of Object.keys(flat.attributes))if(!['position','normal','color'].includes(key))flat.deleteAttribute(key);parts.push(flat);if(flat!==g)g.dispose();o.geometry.dispose();o.material.dispose();});
  const merged=mergeGeometries(parts);parts.forEach(g=>g.dispose());merged.computeBoundingBox();const b=merged.boundingBox,center=b.getCenter(new T.Vector3());merged.translate(-center.x,-b.min.y,-center.z);merged.scale(.5,.5,.5);geometries.add(merged);
  const modelMat=new T.MeshStandardMaterial({vertexColors:true,roughness:.72});materials.add(modelMat);tools=new T.InstancedMesh(merged,modelMat,10);tools.castShadow=true;tools.receiveShadow=true;scene.add(tools);
  const dummy=new T.Object3D();positions.forEach((p,i)=>{dummy.position.set(p.x,0,p.z);dummy.rotation.y=p.rotation;dummy.updateMatrix();tools.setMatrixAt(i,dummy.matrix);const m=mat('#91a2a5');markers.push(box(.24,.10,5.4,m,p.x<0?-3.65:3.65,.04,p.z));const halo=add(new T.RingGeometry(3.05,3.48,64),new T.MeshBasicMaterial({color:'#91a2a5',transparent:true,opacity:.85,toneMapped:false,side:T.DoubleSide,depthWrite:false}),p.x,.06,p.z);halo.rotation.x=-Math.PI/2;materials.add(halo.material);halos.push(halo);const lamp=add(new T.SphereGeometry(.32,16,12),mat('#91a2a5'),p.x<0?-4:4,2.9,p.z);lamps.push(lamp);label(p.id,5.5).position.set(p.x,3.6,p.z);});tools.instanceMatrix.needsUpdate=true;
  listen(renderer.domElement,'pointerdown',e=>{down={x:e.clientX,y:e.clientY};});
  function pick(e){const r=renderer.domElement.getBoundingClientRect();pointer.set((e.clientX-r.left)/r.width*2-1,-(e.clientY-r.top)/r.height*2+1);ray.setFromCamera(pointer,camera);const hit=ray.intersectObject(tools)[0];if(hit)onSelect(positions[hit.instanceId].id);}
  listen(renderer.domElement,'pointerup',e=>{if(down&&Math.hypot(down.x-e.clientX,down.y-e.clientY)<6)pick(e);down=null;});
  listen(renderer.domElement,'pointermove',e=>{if(e.pointerType==='mouse'&&!e.buttons)pick(e);});
  listen(renderer.domElement,'pointercancel',()=>{down=null;});
  resize();setCamera(true);return api;
 }catch(e){dispose();throw e;}
 function frame(time){if(!active||dead)return;raf=requestAnimationFrame(frame);const actualDt=last?(time-last)/1000:0;const dt=Math.min(actualDt,.05);last=time;setCamera();let v={x:0,z:0};
  if(state.mode==='auto'){
   state.pause=Math.max(0,state.pause-dt);if(!state.pause){const q=route[state.waypoint];const dx=q.x-state.x,dz=q.z-state.z,dist=Math.hypot(dx,dz);if(dist<.12){state.waypoint=(state.waypoint+1)%route.length;state.pause=2;}else v={x:dx/dist,z:dz/dist};}
  }else if(state.view==='follow')v=input;
  const next=move(state,v,dt),distance=Math.hypot(next.x-state.x,next.z-state.z);if(distance>.0001){state.heading=Math.atan2(next.x-state.x,next.z-state.z);walkPhase+=dt*8;}state.x=next.x;state.z=next.z;
  avatar.position.set(state.x,0,state.z);avatar.rotation.y=state.heading;limbs.forEach((l,i)=>{l.rotation.x=distance>.0001?Math.sin(walkPhase)*(i<2?1:-1)*(i%2?-.35:.45):0;});
  if(state.view==='follow'){
   // Fixed aisle-aligned trailing camera stays inside the clear corridor.
   camera.position.set(state.x*.45,3.5,Math.min(21.5,state.z+5));camera.lookAt(state.x,1.2,state.z-1.4);
  }else controls.update();
  markers.forEach((m,i)=>{const s=status(fleet.find(f=>f.equipmentId===positions[i].id)),level=glowLevel(s,time,reduced);for(const mesh of [m,lamps[i]]){mesh.material.color.set(s.color);mesh.material.emissive.set(s.color);mesh.material.emissiveIntensity=level*2.2;}halos[i].material.color.set(s.color);halos[i].material.opacity=.12+level*.88;});
  renderer.render(scene,camera);frames++;elapsed+=actualDt;
  if(time-metricsTime>1000){onMetrics({fps:elapsed?Math.round(frames/elapsed):0,calls:renderer.info.render.calls,triangles:renderer.info.render.triangles});frames=0;elapsed=0;metricsTime=time;}
 }
}
