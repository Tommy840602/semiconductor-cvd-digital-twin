export const ids = Array.from({length:10},(_,i)=>`TEL-CVDTIN-08A-${String(i+1).padStart(2,'0')}`);
export const positions = ids.map((id,i)=>({id,x:i<5?-7.6:7.6,z:(i%5-2)*7.8,rotation:i<5?Math.PI/2:-Math.PI/2}));
export const bounds={minX:-3,maxX:3,minZ:-20,maxZ:20};
export const route=[...positions.slice(0,5).map(p=>({x:-2,z:p.z})),...positions.slice(5).reverse().map(p=>({x:2,z:p.z}))];
export {machineStatus as status} from '../statusGlow.js';
export function move(position,vector,dt,obstacles=[]) {
 const length=Math.hypot(vector.x,vector.z);if(!length)return {...position};
 const scale=2.2*Math.min(Math.max(dt,0),.05)/Math.max(1,length);
 const next={...position};
 // Small axis-separated steps slide against boundaries without tunnelling.
 for(const axis of ['x','z']){
  const v=next[axis]+vector[axis]*scale;
  const candidate={...next,[axis]:v};
  if(candidate.x<bounds.minX||candidate.x>bounds.maxX||candidate.z<bounds.minZ||candidate.z>bounds.maxZ)continue;
  if(obstacles.some(o=>candidate.x>o.minX-.3&&candidate.x<o.maxX+.3&&candidate.z>o.minZ-.3&&candidate.z<o.maxZ+.3))continue;
  next[axis]=v;
 }
 return next;
}
export function nearestWaypoint(p){return route.reduce((best,q,i)=>Math.hypot(p.x-q.x,p.z-q.z)<Math.hypot(p.x-route[best].x,p.z-route[best].z)?i:best,0);}
export function createState(){return {x:0,z:18,heading:Math.PI,mode:'auto',view:'overview',waypoint:0,pause:0};}

// Reachable interaction strip on the aisle side of each three-port frontage.
export function nearbyLoadPort(state) {
 const automatic=state.mode==='auto';
 if(!automatic&&(state.mode!=='manual'||state.view!=='follow'))return null;
 // Patrol follows x = ±2; allow the small arrival tolerance used by the robot.
 const minDistance=automatic?1.8:2.25;
 if(!Number.isFinite(state.x)||!Number.isFinite(state.z)||Math.abs(state.x)<minDistance||Math.abs(state.x)>bounds.maxX)return null;
 return positions.find(p=>Math.sign(p.x)===Math.sign(state.x)&&Math.abs(p.z-state.z)<=1.8)?.id||null;
}
