import * as T from 'three';
import {mergeGeometries} from 'three/addons/utils/BufferGeometryUtils.js';

// Original procedural materials: no downloaded textures or external asset licenses.
export function buildEnvironment(scene,{materials,geometries,textures}) {
 const batches=new Map();
 const material=(color,options={})=>{const m=new T.MeshStandardMaterial({color,roughness:.65,...options});materials.add(m);return m;};
 const panel=material('#dbe5e4'),trim=material('#829a9e',{metalness:.45,roughness:.4}),base=material('#487275');
 const glass=material('#558790',{metalness:.25,roughness:.22}),light=material('#f4ffff',{emissive:'#d8f5ff',emissiveIntensity:1.3}),filter=material('#adbcbf');
 function box(w,h,d,m,x,y,z,roof=false){
  const key=String(m.id)+roof;if(!batches.has(key))batches.set(key,{m,roof,parts:[]});
  batches.get(key).parts.push(new T.BoxGeometry(w,h,d).translate(x,y,z));
 }
 const canvas=document.createElement('canvas');canvas.width=canvas.height=256;const c=canvas.getContext('2d');
 c.fillStyle='#bacccc';c.fillRect(0,0,256,256);c.fillStyle='#dae4e2';c.fillRect(3,3,250,250);
 c.strokeStyle='#eef4f1';c.strokeRect(5,5,246,246);
 for(let x=20;x<250;x+=18)for(let y=20;y<250;y+=18){c.fillStyle='#9aafaf';c.beginPath();c.arc(x,y,2,0,Math.PI*2);c.fill();c.fillStyle='#f3f7f6';c.fillRect(x-2,y+2,4,1);}
 const texture=new T.CanvasTexture(canvas);texture.colorSpace=T.SRGBColorSpace;texture.wrapS=texture.wrapT=T.RepeatWrapping;texture.repeat.set(24,36);texture.anisotropy=4;textures.add(texture);
 const floor=material('#ffffff',{map:texture,roughness:.5,metalness:.12});
 box(29,.2,44,floor,0,-.14,0);
 // Inset service-area boundaries and central walking lane.
 const marking=material('#d8c88f');for(const x of [-3.6,3.6])box(.06,.012,41,marking,x,-.028,0);
 for(let z=-19;z<=19;z+=3)box(.08,.014,.7,panel,0,-.025,z);
 for(const x of [-14.3,14.3]){
  box(.2,4.8,44,panel,x,2.25,0);box(.24,.2,44,base,x,.1,0);box(.26,.08,44,trim,x,3.85,0);
  for(let z=-21;z<=21;z+=3){box(.24,4.6,.035,trim,x,2.25,z);}
  for(let z=-17;z<=17;z+=8.5){
   const inside=x-Math.sign(x)*.14;box(.05,1.2,3.1,trim,inside,2.1,z);box(.06,1.04,2.92,glass,inside-Math.sign(x)*.04,2.1,z);
   box(.08,1.2,.04,trim,inside-Math.sign(x)*.08,2.1,z);
  }
 }
 // End-wall airlock door and observation windows; cut away in overview.
 for(const z of [-21.9,21.9]){
  box(29,4.6,.16,panel,0,2.2,z,true);box(2.8,3.1,.2,trim,0,1.5,z-Math.sign(z)*.13,true);
  box(2.58,2.95,.22,panel,0,1.5,z-Math.sign(z)*.15,true);
  for(const x of [-.64,.64])box(.87,1.05,.24,glass,x,1.85,z-Math.sign(z)*.18,true);
  box(.035,2.96,.25,trim,0,1.5,z-Math.sign(z)*.2,true);
  box(1.4,.3,.25,base,0,3.35,z-Math.sign(z)*.17,true);
 }
 for(const z of [-19,-11,-3,5,13,21])for(const x of [-12.6,12.6])box(.32,4.8,.32,trim,x,2.25,z);
 box(29,.12,44,panel,0,4.94,0,true);
 // Suspended FFU grid and LED troffers are shown from inside the room only.
 for(let z=-20;z<=20;z+=4){
  box(28,.09,.06,trim,0,4.72,z,true);
  for(const x of [-10,-5,0,5,10]){
   box(2.9,.12,2.5,panel,x,4.72,z+1.6,true);box(2.65,.06,2.23,filter,x,4.63,z+1.6,true);
   for(let n=-1;n<=1;n+=.25)box(2.6,.025,.025,trim,x,4.59,z+1.6+n,true);
   box(2.9,.09,.4,light,x,4.58,z-.45,true);
  }
 }
 const roof=new T.Group();scene.add(roof);
 for(const {m,parts,roof:isRoof} of batches.values()){
  const geometry=mergeGeometries(parts);parts.forEach(p=>p.dispose());geometries.add(geometry);
  const mesh=new T.Mesh(geometry,m);mesh.receiveShadow=true;(isRoof?roof:scene).add(mesh);
 }
 return roof;
}
