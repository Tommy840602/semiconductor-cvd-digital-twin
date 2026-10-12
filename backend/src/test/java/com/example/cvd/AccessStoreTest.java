package com.example.cvd;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.PlatformTransactionManager;
class AccessStoreTest {
 @Test void initializedDatabaseDoesNotReadSecretOrResetUsers(){
  var pg=mock(JdbcTemplate.class);
  when(pg.queryForObject("SELECT count(*) FROM auth_bootstrap WHERE id=1",Long.class)).thenReturn(1L);
  var store=new AccessStore(pg,mock(PlatformTransactionManager.class),"/missing-secret");
  assertDoesNotThrow(store::initialize);
  verify(pg,never()).update(anyString(),any(Object[].class));
  verify(pg,never()).queryForObject("SELECT count(*) FROM app_users WHERE username='cvd.leader'",Long.class);
 }
 @Test void bootstrapCollisionNeverOverwritesExistingAccount(){
  var pg=mock(JdbcTemplate.class);
  when(pg.queryForObject("SELECT count(*) FROM auth_bootstrap WHERE id=1",Long.class)).thenReturn(0L);
  when(pg.queryForObject("SELECT count(*) FROM app_users WHERE username='cvd.leader'",Long.class)).thenReturn(1L);
  var store=new AccessStore(pg,mock(PlatformTransactionManager.class),"/missing-secret");
  assertThrows(IllegalStateException.class,store::initialize);
  verify(pg,never()).update(anyString(),any(Object[].class));
 }
 @Test void rejectsBcryptTruncationAndMalformedNames(){
  assertFalse(AccessStore.validPassword("short"));assertFalse(AccessStore.validPassword("中".repeat(25)));
  assertTrue(AccessStore.validPassword("a".repeat(12)));assertFalse(AccessStore.validUsername("../leader"));
 }
}
