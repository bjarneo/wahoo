import { createServer } from "vite";

const vite = await createServer({ server: { middlewareMode: true }, appType: "custom" });
vite.middlewares.use(async (req, res, next) => {
  if (req.url?.startsWith("/__wahoo_ssr")) {
    try {
      const mod = await vite.ssrLoadModule("/src/entry-server.tsx");
      res.statusCode = 200;
      res.setHeader("content-type", "text/html; charset=utf-8");
      res.end(mod.render());
      return;
    } catch (error) {
      vite.ssrFixStacktrace(error);
      next(error);
      return;
    }
  }
  next();
});
vite.listen(4173);
console.log("Wahoo SSR worker listening on http://127.0.0.1:4173");
