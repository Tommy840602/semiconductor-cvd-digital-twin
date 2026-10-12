package com.example.cvd;
import java.net.URI;
import java.nio.file.*;
import java.util.*;
import org.springframework.core.env.Environment;
import org.springframework.stereotype.Component;

@Component
public class EmailConfig {
 final boolean allowed; final String clientId,redirect,publicBase,secretFile,keyFile; final int offlineSeconds,recoverySeconds;
 public EmailConfig(Environment env){allowed=Boolean.parseBoolean(env.getProperty("EMAIL_ENABLED","false"));clientId=env.getProperty("OUTLOOK_CLIENT_ID","");redirect=env.getProperty("OAUTH_REDIRECT_URI","");publicBase=env.getProperty("PUBLIC_BASE_URL","");secretFile=env.getProperty("OUTLOOK_CLIENT_SECRET_FILE","");keyFile=env.getProperty("EMAIL_TOKEN_KEY_FILE","");offlineSeconds=Math.max(1,env.getProperty("EMAIL_OFFLINE_SECONDS",Integer.class,60));recoverySeconds=Math.max(1,env.getProperty("EMAIL_RECOVERY_SECONDS",Integer.class,10));}
 boolean configured(){return !clientId.isBlank()&&!redirect.isBlank()&&!secretFile.isBlank()&&!keyFile.isBlank();}
 static String url(String value,boolean callback){
  try{URI u=URI.create(value);boolean local=Set.of("localhost","127.0.0.1").contains(u.getHost()==null?"":u.getHost());
   if(u.getHost()==null||u.getUserInfo()!=null||u.getQuery()!=null||u.getFragment()!=null||!("https".equals(u.getScheme())||local&&"http".equals(u.getScheme()))||(!callback&&!Set.of("","/").contains(u.getPath()))||(callback&&!"/oauth/outlook".equals(u.getPath())))throw new IllegalArgumentException();
   return callback?u.toString():u.toString().replaceAll("/$","");
  }catch(Exception e){throw new IllegalArgumentException(callback?"Callback 必須為 HTTPS /oauth/outlook（本機可用 HTTP）":"網站基底網址必須為 HTTPS origin（本機可用 HTTP）");}
 }
 String secret(){try{return Files.readString(Path.of(secretFile)).strip();}catch(Exception e){throw new IllegalStateException("Outlook secret 尚未設定");}}
 byte[] key(){try{byte[] key=Base64.getDecoder().decode(Files.readString(Path.of(keyFile)).strip());if(key.length!=32)throw new IllegalArgumentException();return key;}catch(Exception e){throw new IllegalStateException("Email 加密金鑰需為 base64 32 bytes");}}
}
