package com.example.cvd;
import org.junit.jupiter.api.Test;
import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.node.*;
import static org.junit.jupiter.api.Assertions.*;
class VisionValidationTest {
 ObjectMapper m=new ObjectMapper();
 @Test void missingFieldsHashBoundsAndScale(){var in=m.createObjectNode().put("hash","a".repeat(64)).put("kind","surface").put("width",64).put("height",64);in.putNull("calibration");var r=m.createObjectNode().put("inputHash","a".repeat(64)).put("overlayHash","b".repeat(64)).put("kind","surface").put("width",64).put("height",64).put("algorithmVersion","opencv-rules-v1").put("opencvVersion","4.12.0").put("quality","review");r.putObject("parameters");r.putArray("candidates").add("particle");var d=r.putArray("defects").addObject().put("id",1).put("type","particle").put("x",1).put("y",2).put("width",5).put("height",5).put("sizePx",5).put("areaPx",20);assertTrue(VisionApi.validResult(r,in));d.put("x",63);assertFalse(VisionApi.validResult(r,in));d.put("x",1).put("sizeUm",1);assertFalse(VisionApi.validResult(r,in));d.remove("sizeUm");d.put("areaPx",Double.NaN);assertFalse(VisionApi.validResult(r,in));d.put("areaPx",20);r.remove("inputHash");assertFalse(VisionApi.validResult(r,in));assertFalse(VisionApi.validResult(m.createObjectNode(),in));}
}
