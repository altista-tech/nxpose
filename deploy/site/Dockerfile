FROM squidfunk/mkdocs-material:9.6 AS build
WORKDIR /docs
COPY site/ .
RUN pip install pillow cairosvg
RUN mkdocs build

FROM nginx:mainline-alpine-slim
COPY --from=build /docs/site /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
