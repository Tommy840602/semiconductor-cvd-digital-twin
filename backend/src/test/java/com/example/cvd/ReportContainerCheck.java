package com.example.cvd;
import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.nio.file.*;
/** Standalone smoke check for the deployed JRE and bundled CJK font; no database or API calls. */
public class ReportContainerCheck {
 public static void main(String[] args)throws Exception{var json=new ObjectMapper();var fixture=json.readTree(Files.readString(Path.of("/probe/manifest.json")));var renderer=new ReportRenderer(json);if(!renderer.chartFont.canDisplay('晶')||!renderer.chartFont.canDisplay('語'))throw new IllegalStateException("missing glyph");renderer.render(Path.of("/probe/out"),(ObjectNode)fixture.path("snapshot"),(ObjectNode)fixture.path("analysis"),"container-fixture");System.out.println("PASS Linux JRE, bundled CJK glyphs, PNG charts and XLSX generation");}
}
