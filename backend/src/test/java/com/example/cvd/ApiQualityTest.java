package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import org.junit.jupiter.api.Test;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.data.redis.core.*;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.web.context.request.*;
class ApiQualityTest {
 @Test void sameVersionCacheMustNotHideChangedCommunicationQuality() throws Exception {
  var request=new MockHttpServletRequest("GET","/api/equipment/"+Equipment.FIRST);
  RequestContextHolder.setRequestAttributes(new ServletRequestAttributes(request));
  try {
   var mapper=new ObjectMapper();var db=mock(DataStore.class);
   String authoritative="{\"equipmentId\":\""+Equipment.FIRST+"\",\"version\":7,\"communicationQuality\":\"offline\"}";
   when(db.snapshot()).thenReturn(authoritative);when(db.parse(anyString())).thenAnswer(i->mapper.readTree((String)i.getArgument(0)));
   var redis=mock(StringRedisTemplate.class);ValueOperations<String,String> values=mock(ValueOperations.class);when(redis.opsForValue()).thenReturn(values);
   for(String quality:new String[]{"",",\"communicationQuality\":\"good\""}) {
    when(values.get(anyString())).thenReturn("{\"equipmentId\":\""+Equipment.FIRST+"\",\"version\":7"+quality+"}");
    assertEquals("offline",new Api(db,redis,mock(AccessStore.class)).equipment().path("communicationQuality").asText());
   }
   verify(values,times(2)).set("cvd:"+Equipment.FIRST+":latest",authoritative);
  } finally {RequestContextHolder.resetRequestAttributes();}
 }
}
