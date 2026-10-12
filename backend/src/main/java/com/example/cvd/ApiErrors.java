package com.example.cvd;
import java.util.Map;
import org.springframework.http.*;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;
@RestControllerAdvice
public class ApiErrors {
 @ExceptionHandler(ResponseStatusException.class)
 ResponseEntity<Map<String,Object>> status(ResponseStatusException e){return ResponseEntity.status(e.getStatusCode()).body(SystemMessages.response(e.getReason()==null?"請求無法完成":e.getReason()));}
 @ExceptionHandler(org.springframework.dao.DataAccessException.class)
 ResponseEntity<Map<String,Object>> storage(Exception e){return ResponseEntity.status(503).body(SystemMessages.response("資料服務暫時無法使用"));}
}
