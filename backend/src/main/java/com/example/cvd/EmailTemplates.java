package com.example.cvd;

import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.util.*;

final class EmailTemplates {
 static final int VERSION=1;
 static Map<String,String> render(Map<String,Object> job,String base,String locale){
  String type=job.get("type").toString();
  String[] titles=switch(type){
   case "fault" -> new String[]{"PM 故障","PM fault","PM 故障"};
   case "fault-cleared" -> new String[]{"PM 故障解除","PM fault cleared","PM 故障解除"};
   case "offline" -> new String[]{"設備持續失聯","Equipment remains offline","装置の通信断が継続"};
   case "online" -> new String[]{"設備通訊恢復","Equipment communication restored","装置通信復旧"};
   case "review" -> new String[]{"補做／報廢提案待 PE 審核","Recovery / scrap proposal awaiting PE review","追加処理／廃棄提案の PE 審査待ち"};
   case "result" -> new String[]{"異常處置結果待 PE 確認","Disposition result awaiting PE confirmation","異常処置結果の PE 確認待ち"};
   case "release" -> new String[]{"整台機台待 Leader 放行","Equipment awaiting Leader release","装置全体の Leader 解除待ち"};
   default -> new String[]{"設備通知","Equipment notification","装置通知"};
  };
  int i="en".equals(locale)?1:"ja".equals(locale)?2:0;
  String eq=Equipment.require(job.get("equipment_id").toString()),subject=job.get("subject_id").toString();
  String url=EmailConfig.url(base,false)+"/?equipment="+eq+(Set.of("review","result","release").contains(type)?"&view=disposition":"")+(Set.of("review","result").contains(type)?"&case="+java.net.URLEncoder.encode(subject,java.nio.charset.StandardCharsets.UTF_8):"");
  String time=((java.sql.Timestamp)job.get("source_time")).toInstant().atZone(ZoneId.of("Asia/Taipei")).format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss 'UTC'XXX"));
  String brand=new String[]{"CVD 模擬平台","CVD simulation platform","CVD シミュレーションプラットフォーム"}[i];
  String equipment=new String[]{"機台","Equipment","装置"}[i],incident=new String[]{"PM／案件","PM / case","PM／案件"}[i],eventTime=new String[]{"事件時間","Event time","イベント日時"}[i];
  String footer=new String[]{"請登入平台查看目前狀態；所有核准、處置與恢復操作仍需通過平台權限驗證。","Sign in to view the current state. All approvals, disposition and resume actions still require platform authorization.","ログインして現在の状態を確認してください。承認、処置、再開の操作には、引き続きプラットフォームの権限確認が必要です。"}[i];
  return Map.of("subject","["+brand+"] "+eq+" "+titles[i],"text",brand+"\n"+titles[i]+"\n"+equipment+": "+eq+"\n"+incident+": "+subject+"\n"+eventTime+": "+time+" (Asia/Taipei)\n\n"+url+"\n"+footer);
 }
}
