FROM squidfunk/mkdocs-material:9.6 AS docs-build
WORKDIR /docs
COPY site/mkdocs.yml .
COPY site/docs docs/
COPY site/overrides overrides/
RUN pip install pillow cairosvg
RUN mkdocs build

FROM nginx:mainline-alpine-slim
COPY site/landing/ /usr/share/nginx/html/
COPY --from=docs-build /docs/site /usr/share/nginx/html/docs/
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
