/* eslint-disable @typescript-eslint/no-var-requires */
const path = require('path');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const webpack = require('webpack');

module.exports = {
  mode: 'development',
  entry: path.resolve(__dirname, 'e2e/harness/index.tsx'),
  output: {
    path: path.resolve(__dirname, 'dist-test'),
    filename: 'bundle.js',
    publicPath: '/',
  },
  resolve: {
    extensions: ['.ts', '.tsx', '.js', '.jsx', '.json'],
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: {
          loader: 'ts-loader',
          options: {
            configFile: path.resolve(__dirname, 'e2e/tsconfig.json'),
            transpileOnly: true,
          },
        },
        exclude: /node_modules/,
      },
      {
        test: /\.css$/,
        use: ['style-loader', 'css-loader'],
      },
      {
        test: /\.(png|jpg|jpeg|gif|svg|woff|woff2|eot|ttf|otf)$/,
        type: 'asset/resource',
      },
    ],
  },
  plugins: [
    new HtmlWebpackPlugin({
      template: path.resolve(__dirname, 'e2e/harness/index.html'),
    }),
    // Swap the real SDK for our mock
    new webpack.NormalModuleReplacementPlugin(
      /@openshift-console\/dynamic-plugin-sdk$/,
      path.resolve(__dirname, 'e2e/harness/sdk-mock.ts'),
    ),
  ],
  devServer: {
    port: 9002,
    historyApiFallback: true,
    hot: false,
    liveReload: false,
  },
  devtool: 'cheap-module-source-map',
};
