package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import org.junit.jupiter.api.*;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.web.context.request.*;
import org.springframework.web.server.ResponseStatusException;
import com.fasterxml.jackson.databind.ObjectMapper;
class EquipmentTest {
 @AfterEach void clear(){RequestContextHolder.resetRequestAttributes();}
 @Test void validIdentityAndMissingScope(){assertEquals(10,Equipment.IDS.size());assertThrows(ResponseStatusException.class,()->Equipment.require("PM1"));assertThrows(ResponseStatusException.class,()->Equipment.require("TEL-CVDTIN-08A-11"));assertThrows(ResponseStatusException.class,Equipment::current);}
 @Test void requestScope(){var r=new MockHttpServletRequest("GET","/api/equipment/TEL-CVDTIN-08A-02/history");RequestContextHolder.setRequestAttributes(new ServletRequestAttributes(r));assertEquals("TEL-CVDTIN-08A-02",Equipment.current());}
 @Test void recipeParameters()throws Exception{var m=new ObjectMapper();RecipeApi.validate(m.readTree("{\"stepSeconds\":{\"preheat\":1,\"stabilize\":2,\"deposition\":3,\"purge\":4}}"));assertThrows(ResponseStatusException.class,()->RecipeApi.validate(m.readTree("{\"stepSeconds\":{\"preheat\":1,\"stabilize\":2,\"deposition\":0,\"purge\":4}}")));}
}
