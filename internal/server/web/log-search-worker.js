'use strict';
// Run user expressions off the UI thread; the caller terminates stalled searches.
self.onmessage=event=>{
 try{
  const {texts,pattern,sensitive}=event.data,expression=new RegExp(pattern,sensitive?'gm':'gim'),ranges=[];let offset=0,limited=false;
  for(const text of texts){
   expression.lastIndex=0;let match;
   while((match=expression.exec(text))!==null){
    if(match[0].length){ranges.push({start:offset+match.index,length:match[0].length});if(ranges.length>=2000){limited=true;break}}
    else expression.lastIndex++;
   }
   if(limited)break;offset+=text.length;
  }
  self.postMessage({ranges,limited});
 }catch{self.postMessage({error:true})}
};
