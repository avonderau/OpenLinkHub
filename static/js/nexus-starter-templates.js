'use strict';
// Stock-inspired Nexus starter screens. Only actions supported by the
// fork's screen configuration schema are used here.
globalThis.NEXUS_STARTER_TEMPLATES = Object.freeze([
  {
    id:'system-overview',
    name:'System Overview',
    description:'CPU and GPU temperature with memory use in one glance.',
    widgets:[
      {type:'metric',metric:'cpu.temperature',x:0,y:0,width:208,height:48,settings:{label:'CPU',unit:'°C',backgroundColor:'#173344',textColor:'#eaf6ff',borderColor:'#30809b'}},
      {type:'metric',metric:'gpu.temperature',x:216,y:0,width:208,height:48,settings:{label:'GPU',unit:'°C',backgroundColor:'#173344',textColor:'#eaf6ff',borderColor:'#30809b'}},
      {type:'bar',metric:'memory.utilization',x:432,y:0,width:208,height:48,settings:{label:'MEM',unit:'%',backgroundColor:'#173344',textColor:'#eaf6ff',borderColor:'#30809b'}}
    ]
  },
  {
    id:'performance-bars',
    name:'Performance Bars',
    description:'CPU, GPU and memory utilization in matching bar widgets.',
    widgets:[
      {type:'bar',metric:'cpu.utilization',x:0,y:0,width:208,height:48,settings:{label:'CPU',unit:'%',backgroundColor:'#263144',textColor:'#ffffff',borderColor:'#6c91c7'}},
      {type:'bar',metric:'gpu.utilization',x:216,y:0,width:208,height:48,settings:{label:'GPU',unit:'%',backgroundColor:'#263144',textColor:'#ffffff',borderColor:'#6c91c7'}},
      {type:'bar',metric:'memory.utilization',x:432,y:0,width:208,height:48,settings:{label:'RAM',unit:'%',backgroundColor:'#263144',textColor:'#ffffff',borderColor:'#6c91c7'}}
    ]
  },
  {
    id:'network-and-storage',
    name:'Network & Storage',
    description:'Live network and disk transfer rates in four columns.',
    widgets:[
      {type:'metric',metric:'network.rx_rate',x:0,y:0,width:152,height:48,settings:{label:'NET IN',backgroundColor:'#143a3b',textColor:'#dafbf0'}},
      {type:'metric',metric:'network.tx_rate',x:160,y:0,width:152,height:48,settings:{label:'NET OUT',backgroundColor:'#143a3b',textColor:'#dafbf0'}},
      {type:'metric',metric:'disk.read_rate',x:320,y:0,width:152,height:48,settings:{label:'DISK R',backgroundColor:'#183646',textColor:'#e2f3ff'}},
      {type:'metric',metric:'disk.write_rate',x:480,y:0,width:160,height:48,settings:{label:'DISK W',backgroundColor:'#183646',textColor:'#e2f3ff'}}
    ]
  },
  {
    id:'media-deck',
    name:'Media Controls',
    description:'Six ready-to-use previous, play/pause, next, stop and volume buttons.',
    widgets:[
      {type:'button',x:0,y:0,width:96,height:48,action:'media',target:'previous-track',settings:{icon:'previous-track',style:'solid',backgroundColor:'#253248',textColor:'#ffffff',borderColor:'#4c668f'}},
      {type:'button',x:104,y:0,width:96,height:48,action:'media',target:'play-pause',settings:{icon:'play-pause',style:'solid',backgroundColor:'#294d69',textColor:'#ffffff',borderColor:'#589dc0'}},
      {type:'button',x:208,y:0,width:96,height:48,action:'media',target:'next-track',settings:{icon:'next-track',style:'solid',backgroundColor:'#253248',textColor:'#ffffff',borderColor:'#4c668f'}},
      {type:'button',x:312,y:0,width:96,height:48,action:'media',target:'stop',settings:{icon:'stop',style:'solid',backgroundColor:'#59333b',textColor:'#ffffff',borderColor:'#ba6978'}},
      {type:'button',x:416,y:0,width:96,height:48,action:'media',target:'volume-down',settings:{icon:'volume-down',style:'solid',backgroundColor:'#253248',textColor:'#ffffff',borderColor:'#4c668f'}},
      {type:'button',x:520,y:0,width:120,height:48,action:'media',target:'volume-up',settings:{icon:'volume-up',style:'solid',backgroundColor:'#253248',textColor:'#ffffff',borderColor:'#4c668f'}}
    ]
  },
  {
    id:'navigation',
    name:'Screen Navigation',
    description:'Large previous/next controls with a centered title.',
    widgets:[
      {type:'button',x:0,y:0,width:136,height:48,action:'previous',settings:{label:'',icon:'previous',style:'outline',textColor:'#eaf6ff',borderColor:'#6e9db3'}},
      {type:'text',x:144,y:0,width:352,height:48,settings:{text:'NEXUS',backgroundColor:'#172b3a',textColor:'#eaf6ff',borderColor:'#34576e'}},
      {type:'button',x:504,y:0,width:136,height:48,action:'next',settings:{label:'',icon:'next',style:'outline',textColor:'#eaf6ff',borderColor:'#6e9db3'}}
    ]
  }
]);
