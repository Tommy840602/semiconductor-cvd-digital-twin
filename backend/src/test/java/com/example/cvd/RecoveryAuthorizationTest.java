package com.example.cvd;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import java.util.*;
import org.junit.jupiter.api.*;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.server.ResponseStatusException;

class RecoveryAuthorizationTest {
 @AfterEach void clear(){SecurityContextHolder.clearContext();}
 @Test void departmentAndRoleMatrix(){
  var api=new RecoveryApi(mock(DataStore.class),mock(AccessStore.class),"");
  for(String role:List.of("Operator","Engineer","Leader"))for(String department:Arrays.asList(null,"EE","PE"))for(String required:List.of("EE","PE")){
   var identity=new AccessSessions.Identity("test","test",role,department,1);
   SecurityContextHolder.getContext().setAuthentication(new UsernamePasswordAuthenticationToken(identity,null,List.of()));
   if(!role.equals("Operator")&&required.equals(department))assertDoesNotThrow(()->api.authorize(required));
   else assertEquals(403,assertThrows(ResponseStatusException.class,()->api.authorize(required)).getStatusCode().value());
  }
 }
 @Test void onlyExplicitDepartmentsAccepted(){assertDoesNotThrow(()->AuthApi.validDepartment(null));assertDoesNotThrow(()->AuthApi.validDepartment("EE"));assertDoesNotThrow(()->AuthApi.validDepartment("PE"));for(String value:List.of("Leader","ee","PE ",""))assertThrows(ResponseStatusException.class,()->AuthApi.validDepartment(value));}
}
